package healthcheck

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gammazero/workerpool"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/common/utils"

	"github.com/One-Piecs/proxypool/pkg/proxy"
)

var (
	defaultURLTestTimeout = time.Second * 5

	// testURLsMu 保护 testURLs（可通过 SetTestURLs 配置覆盖）
	testURLsMu sync.RWMutex

	// testURLs 延迟检测（健康检查）的测试地址列表：
	// - 全部为返回 204 的端点（期望状态码判定，200 类端点会白白浪费请求，已剔除）
	// - 国内可达的 204 端点排前面（cp.cloudflare / 小米 miui），海外端点作后备
	// - 不含 gstatic（部署 VPS 上常因访问过度被拒/限流）
	// - 每轮实际只试前 maxTestURLs 个，顺序即优先级
	// 部署环境可用 config `healthcheck_test_urls` 覆盖（见 internal/app/task.go）
	testURLs = []string{
		"https://cp.cloudflare.com/generate_204",    // Cloudflare，国内一般可达
		"https://connect.rom.miui.com/generate_204", // 小米，国内稳定
		"http://www.google.com/generate_204",        // 海外
		"http://bing.com/generate_204",              // 海外
		"http://edge-http.microsoft.com/captiveportal/generate_204",
		"http://clients3.google.com/generate_204",
		"http://apple-cloudkit.com/generate_204",
		"http://play.googleapis.com/generate_204",
	}

	maxRetries = 2
	// 每轮重试最多尝试的 URL 数：健康代理在 2 个 URL 成功后即提前返回，
	// 失效代理无需把全部 URL 试完才判定失败
	maxTestURLs = 4
)

// SetTestURLs 覆盖延迟检测的测试地址列表（config `healthcheck_test_urls`）。
// 传入空列表时恢复默认。调用时机：爬取任务启动前。
func SetTestURLs(urls []string) {
	testURLsMu.Lock()
	defer testURLsMu.Unlock()
	if len(urls) == 0 {
		testURLs = []string{
			"https://cp.cloudflare.com/generate_204",
			"https://connect.rom.miui.com/generate_204",
			"http://www.google.com/generate_204",
			"http://bing.com/generate_204",
			"http://edge-http.microsoft.com/captiveportal/generate_204",
			"http://clients3.google.com/generate_204",
			"http://apple-cloudkit.com/generate_204",
			"http://play.googleapis.com/generate_204",
		}
		return
	}
	testURLs = append([]string(nil), urls...)
}

// getTestURLs 返回当前测试地址列表（副本）
func getTestURLs() []string {
	testURLsMu.RLock()
	defer testURLsMu.RUnlock()
	return append([]string(nil), testURLs...)
}

// CleanBadProxiesWithWorkpool 对代理做延迟检测，返回可用（延迟非 0）的代理列表。
func CleanBadProxiesWithWorkpool(proxies []proxy.Proxy) (cproxies []proxy.Proxy) {
	pool := workerpool.New(healthcheckConcurrency())
	c := make(chan *Stat)
	defer close(c)

	total := len(proxies)
	progress := newProgress(total)

	for _, p := range proxies {
		pp := p
		pool.Submit(func() {
			defer progress.inc()
			delay, err := testDelay(pp)
			if err == nil && delay != 0 {
				RecordHealthResult(pp.Identifier(), true)
				c <- findOrCreateDelayStat(pp, delay)
			} else {
				// 失败同样记录 streak，供失效节点冻结机制使用
				RecordHealthResult(pp.Identifier(), false)
			}
		})
	}

	done := make(chan struct{})
	// 注意：done 仅由下面的 goroutine 关闭（StopWait 完成后），
	// 不能再用 defer close(done)，否则循环退出后二次关闭会 panic
	go func() {
		pool.StopWait()
		close(done)
	}()

	okMap := make(map[string]struct{})
	for { // Note: 无限循环，直到能读取到done
		select {
		case ps := <-c:
			if ps.Delay > 0 {
				okMap[ps.Id] = struct{}{}
			}
		case <-done:
			cproxies = make(proxy.ProxyList, 0, total)
			// check usable proxy
			for i := range proxies {
				if _, ok := okMap[proxies[i].Identifier()]; ok {
					cproxies = append(cproxies, proxies[i])
				}
			}
			return
		}
	}
}

// findOrCreateDelayStat 在锁内查找或创建延迟统计，返回非 nil 的 *Stat。
// 提取为独立函数以避免变量遮蔽 bug（旧实现把 c <- ps 移到 if 外后，
// 发送的是被遮蔽的 nil 外层变量，导致接收端 ps.Delay panic）。
func findOrCreateDelayStat(p proxy.Proxy, delay uint16) *Stat {
	statsLock.Lock()
	defer statsLock.Unlock()

	var ps *Stat
	var ok bool
	if ps, ok = ProxyStats.Find(p); ok {
		ps.UpdatePSDelay(delay)
	} else {
		ps = &Stat{
			Id:    p.Identifier(),
			Delay: delay,
		}
		ProxyStats = append(ProxyStats, *ps)
	}
	return ps
}

// Return 0 for error
func testDelay(p proxy.Proxy) (delay uint16, err error) {
	pmap := proxy.ToClashMap(p)
	if pmap == nil {
		return 0, fmt.Errorf("不支持的代理类型: %s", p.TypeName())
	}

	if p.TypeName() == "vmess" {
		if network, ok := pmap["network"]; ok && network.(string) == "h2" {
			return 0, fmt.Errorf("不支持h2协议的延迟测试")
		}
	}

	clashProxy, err := adapter.ParseProxy(pmap)
	if err != nil {
		return 0, fmt.Errorf("创建代理实例失败: %w", err)
	}

	expectedStatus, _ := utils.NewUnsignedRanges[uint16]("204")
	var lastErr error
	var successCount int
	var totalDelay uint16

	// 智能重试机制
	for retry := 0; retry <= maxRetries; retry++ {
		// 自适应超时：首次使用默认超时，之后递增50%
		timeout := defaultURLTestTimeout
		if retry > 0 {
			timeout = time.Duration(float64(timeout) * 1.5)
		}

		// 遍历测试URL（只取前 maxTestURLs 个）
		urls := getTestURLs()
		for _, testURL := range urls[:min(maxTestURLs, len(urls))] {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			currentDelay, err := clashProxy.URLTest(ctx, testURL, expectedStatus)
			cancel()

			// 如果成功获取延迟
			if err == nil && currentDelay > 0 {
				successCount++
				totalDelay += currentDelay

				// 如果已经有足够的成功测试，返回平均延迟
				if successCount >= 2 {
					return totalDelay / uint16(successCount), nil
				}
				continue
			}

			// 记录错误
			lastErr = err
		}

		// 如果有部分成功的测试，返回平均延迟
		if successCount > 0 {
			return totalDelay / uint16(successCount), nil
		}

		// 如果是最后一次重试，返回错误
		if retry == maxRetries {
			return 0, fmt.Errorf("所有重试均失败: %v", lastErr)
		}
	}

	return 0, fmt.Errorf("测试失败: %v", lastErr)
}
