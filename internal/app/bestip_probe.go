package app

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/One-Piecs/proxypool/config"
	"github.com/One-Piecs/proxypool/internal/cache"
	"github.com/One-Piecs/proxypool/log"
	"github.com/One-Piecs/proxypool/pkg/proxy"
	"github.com/gammazero/workerpool"
	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/common/utils"
)

// bestipProbeTestURL 优选 IP 健康检查的数据往返测试地址：
// 隧道建立后经代理发起真实 HTTP 请求，校验 204 状态码，
// 确认数据能端到端转发（而非仅协议握手成功）。
// 默认 cp.cloudflare.com/generate_204（http/https 均返回 204），可经 bestip_probe.test_url 覆盖。
const bestipProbeTestURL = "http://cp.cloudflare.com/generate_204"

// 背景：候选 best ip:port 本质上是一个 SNI proxy（SNI 转发入口）——它只读 TLS
// ClientHello 中的 SNI 字段，把原始 TCP 流路由到对应域名源站，不解析应用层协议。
// 因此健康检查用任一 TLS 承载协议（这里选 vless，凭据齐全、mihomo 支持完善）做
// 完整握手 + 数据往返验证：能通说明该入口可端到端转发 TLS 数据（对 vless/vmess/
// trojan 等 TLS 承载协议均有效），标记 BestNode.Healthy 作为通用可用性。
//
//   - 探测的 sni 参数（= proxy_info[country]["vless"]["host"]）就是喂给 SNI proxy
//     的路由键，必须指向真实可用的 vless 源站域名
//   - 构造参数与 genClashVlessUrl 输出完全一致，保证"探测通过 = 该格式订阅可用"

// ipv6ProbeOK 缓存本机 IPv6 出口检测结果（一次探测周期内只测一次）。
var (
	ipv6CheckOnce sync.Once
	ipv6ProbeOK   bool
)

// ipv6TestAddrs IPv6 出口检测目标（知名公共 DNS 的 443 端口，任一可达即认为有 IPv6 出口）
var ipv6TestAddrs = []string{
	"[2606:4700:4700::1111]:443", // Cloudflare DNS
	"[2400:3200::1]:443",         // AliDNS
	"[2001:4860:4860::8888]:443", // Google DNS
}

// checkIPv6Reachable 检测本机是否具备 IPv6 出口（TCP 连公共 IPv6 地址）。
// 无出口时 IPv6 节点探测必然 network unreachable，探测前据此跳过。
func checkIPv6Reachable() bool {
	ipv6CheckOnce.Do(func() {
		for _, addr := range ipv6TestAddrs {
			conn, err := net.DialTimeout("tcp6", addr, 2*time.Second)
			if err == nil {
				_ = conn.Close()
				ipv6ProbeOK = true
				log.Infoln("bestip probe: IPv6 egress detected (%s)", addr)
				return
			}
		}
		log.Infoln("bestip probe: no IPv6 egress on this host")
	})
	return ipv6ProbeOK
}

// probeBestIPCountry 选择健康检查用国家凭据：配置指定优先，缺省取 proxy_info 中
// 第一个配置了 vless 段的国家（map 遍历顺序不定，但任意一个含 vless 凭据的国家即可，
// 因为标记的是 ip:port 的入口可用性，与具体凭据无关）。
func probeBestIPCountry(cfg *config.BestIPProbeConfig) (string, bool) {
	if cfg != nil && cfg.Country != "" {
		if _, ok := config.Config().ProxyInfo[cfg.Country]["vless"]; ok {
			return cfg.Country, true
		}
		return "", false
	}
	for country, types := range config.Config().ProxyInfo {
		if _, ok := types["vless"]; ok {
			return country, true
		}
	}
	return "", false
}

// probeBestIPNode 探测单个 ip:port 作为优选 IP 入口是否可用：
// 构造临时 vless 出站（server=候选 ip, port=候选 port, uuid/sni/path=源站凭据），
// 经 mihomo `URLTest` 做完整验证：
//
//	L3 隧道建立：TCP + TLS + vless 协议握手（SNI proxy 按 sni 路由到源站）
//	L4 数据往返：经隧道发起真实 HTTP 请求（默认 https://cp.cloudflare.com/generate_204），
//	   收到 204 响应才算可用（仅握手成功但数据不转发视为不可用）
func probeBestIPNode(ip string, port int, uuid, sni, wsPath, testURL string, timeout time.Duration) bool {
	v := &proxy.Vless{
		Base:        proxy.Base{Server: ip, Port: port, Type: "vless"},
		UUID:        uuid,
		ServerName:  sni,
		WSPath:      wsPath,
		Network:     "ws",
		TLS:         true,
		UDP:         true,
		Fingerprint: "chrome",
	}
	clashProxy, err := adapter.ParseProxy(proxy.ToClashMap(v))
	if err != nil {
		log.Errorln("bestip probe ParseProxy(%s:%d): %v", ip, port, err)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	expected, err := utils.NewUnsignedRanges[uint16]("204")
	if err != nil {
		log.Errorln("bestip probe: invalid expected status: %v", err)
		return false
	}
	_, err = clashProxy.URLTest(ctx, testURL, expected)
	if err != nil {
		// IPv6 节点失败时 Debug 级记录错误（不打印 vless:// 链接，避免 proxy_info 凭据泄入日志）
		if IsIPv6(ip) {
			log.Debugln("bestip probe FAILED [%s:%d] url=%s err=%v", ip, port, testURL, err)
		}
		return false
	}
	return true
}

// ProbeAndMarkHealthy 并发探测 best 节点列表（优选 IP 入口健康检查），
// 返回带 Healthy 标记的新切片（不修改入参）。
// 未启用探测或无可探测国家凭据时原样返回。
func ProbeAndMarkHealthy(nodes []cache.BestNode) []cache.BestNode {
	cfg := config.Config().BestIPProbe
	if cfg == nil || !cfg.Enabled() {
		return nodes
	}
	country, ok := probeBestIPCountry(cfg)
	if !ok {
		log.Errorln("bestip probe: no country with vless credentials configured in proxy_info")
		return nodes
	}
	vlessInfo := config.Config().ProxyInfo[country]["vless"]
	uuid, _ := vlessInfo["uuid"].(string)
	sni, _ := vlessInfo["host"].(string)
	wsPath, _ := vlessInfo["path"].(string)
	if uuid == "" || sni == "" {
		log.Errorln("bestip probe: country [%s] vless missing uuid/host", country)
		return nodes
	}
	if wsPath == "" {
		wsPath = "/"
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 20
	}
	timeout := time.Duration(cfg.Timeout) * time.Second
	if cfg.Timeout <= 0 {
		timeout = 5 * time.Second
	}
	testURL := bestipProbeTestURL
	if cfg.TestURL != "" {
		testURL = cfg.TestURL
	}
	if len(nodes) == 0 {
		return nodes
	}

	// 探测前检测本机 IPv6 出口：无出口时 IPv6 节点必然 network unreachable，
	// 直接跳过（避免数百条失败日志刷屏），仅保留一条汇总提示。
	v6OK := checkIPv6Reachable()
	if !v6OK {
		v6Count := 0
		for _, n := range nodes {
			if IsIPv6(n.Ip) {
				v6Count++
			}
		}
		if v6Count > 0 {
			log.Warnln("bestip probe: host has no IPv6 egress, skip %d IPv6 nodes (IPv6 节点健康将显示为 0)", v6Count)
		}
	}

	log.Infoln("bestip probe: testing %d nodes (country=%s, concurrency=%d, timeout=%ds, url=%s)", len(nodes), country, concurrency, int(timeout/time.Second), testURL)

	results := make([]bool, len(nodes))
	wp := workerpool.New(concurrency)
	for i, node := range nodes {
		wp.Submit(func() {
			if IsIPv6(node.Ip) && !v6OK {
				results[i] = false // 无 IPv6 出口，跳过
				return
			}
			results[i] = probeBestIPNode(node.Ip, node.Port, uuid, sni, wsPath, testURL, timeout)
		})
	}
	wp.StopWait()

	marked := make([]cache.BestNode, len(nodes))
	count := 0
	for i, node := range nodes {
		node.Healthy = results[i]
		marked[i] = node
		if node.Healthy {
			count++
		}
	}
	log.Infoln("bestip probe done: %d/%d nodes healthy", count, len(nodes))
	if count == 0 {
		log.Warnln("bestip probe: all nodes failed, check vless origin reachability (host=%s, path=%s) and test url (%s)", sni, wsPath, testURL)
	}
	return marked
}

// ProbeAndMarkNodes 合并执行 best 节点探测（异步编排入口，一次写缓存）：
//   - 先跑优选 IP 健康检查（vless 协议）；若全部失败 → 入口整体不可用，
//     anytls 探测无意义，短路跳过（AnyTLS 保持 false，xxxAnytls 导出为空）
//   - 再跑 anytls 透传探测（sni_probe）
//
// 两个探测各自标记（Healthy / AnyTLS），互不覆盖。
func ProbeAndMarkNodes(nodes []cache.BestNode) []cache.BestNode {
	marked := nodes
	if cfg := config.Config().BestIPProbe; cfg != nil && cfg.Enabled() {
		marked = ProbeAndMarkHealthy(marked)
		if countHealthy(marked) == 0 {
			log.Warnln("bestip probe: all nodes failed, skip anytls probe")
			return marked
		}
	}
	if cfg := config.Config().SniProbe; cfg != nil && cfg.Enabled() {
		marked = ProbeAndMarkAnyTLS(marked)
	}
	return marked
}

// countHealthy 统计健康检查通过的节点数
func countHealthy(nodes []cache.BestNode) int {
	count := 0
	for _, node := range nodes {
		if node.Healthy {
			count++
		}
	}
	return count
}

// filterHealthyNodes 过滤健康检查通过的节点（优选 IP 入口可用）。
// 未启用探测时返回空切片（无探测配置 → vless/vmess/trojan 导出为空）。
func filterHealthyNodes(nodes []cache.BestNode) []cache.BestNode {
	cfg := config.Config().BestIPProbe
	if cfg == nil || !cfg.Enabled() {
		log.Warnln("bestip export: probe disabled (missing bestip_probe config), exporting empty list")
		return nil
	}
	filtered := make([]cache.BestNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Healthy {
			filtered = append(filtered, node)
		}
	}
	return filtered
}
