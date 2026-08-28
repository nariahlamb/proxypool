package healthcheck

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/One-Piecs/proxypool/log"
	"github.com/One-Piecs/proxypool/pkg/proxy"
	C "github.com/metacubex/mihomo/constant"
	"github.com/gammazero/workerpool"
	"github.com/metacubex/mihomo/adapter"
)

var (
	openAITestURLs = []string{
		"https://chat.openai.com",
		"https://api.openai.com",
		"https://platform.openai.com",
	}
	openAITestTimeout = time.Second * 10
	openAIMaxRetries  = 2
)

// openAIVerdict 单次探测的判定结果
type openAIVerdict int

const (
	verdictUnknown openAIVerdict = iota // 无法判定（网络异常/响应格式不明），继续探测
	verdictUnlocked                     // 确认可访问 OpenAI（未封禁）
	verdictBlocked                      // 确认被 OpenAI 封禁/地区不支持
)

func CheckWorkpool(proxies proxy.ProxyList) {
	pool := workerpool.New(500)
	progress := newProgress(len(proxies))

	log.Infoln("ChatGPT Test ON")

	for _, p := range proxies {
		pp := p
		pool.Submit(func() {
			defer progress.inc()
			ok, err := testOpenai(pp)
			if err == nil && ok {
				statsLock.Lock()
				if ps, ok := ProxyStats.Find(pp); ok {
					ps.ChatGPT = true
				}
				statsLock.Unlock()
			}
		})
	}

	pool.StopWait()
	log.Infoln("ChatGPT Test Done")
}

var SupportCountry = []string{"AL", "DZ", "AD", "AO", "AG", "AR", "AM", "AU", "AT", "AZ", "BS", "BD", "BB", "BE", "BZ", "BJ", "BT", "BA", "BW", "BR", "BG", "BF", "CV", "CA", "CL", "CO", "KM", "CR", "HR", "CY", "DK", "DJ", "DM", "DO", "EC", "SV", "EE", "FJ", "FI", "FR", "GA", "GM", "GE", "DE", "GH", "GR", "GD", "GT", "GN", "GW", "GY", "HT", "HN", "HU", "IS", "IN", "ID", "IQ", "IE", "IL", "IT", "JM", "JP", "JO", "KZ", "KE", "KI", "KW", "KG", "LV", "LB", "LS", "LR", "LI", "LT", "LU", "MG", "MW", "MY", "MV", "ML", "MT", "MH", "MR", "MU", "MX", "MC", "MN", "ME", "MA", "MZ", "MM", "NA", "NR", "NP", "NL", "NZ", "NI", "NE", "NG", "MK", "NO", "OM", "PK", "PW", "PA", "PG", "PH", "PL", "PT", "QA", "RO", "RW", "KN", "LC", "VC", "WS", "SM", "ST", "SN", "RS", "SC", "SL", "SG", "SK", "SI", "SB", "ZA", "ES", "LK", "SR", "SE", "CH", "TH", "TG", "TO", "TT", "TN", "TR", "TV", "UG", "AE", "US", "UY", "VU", "ZM", "BO", "BN", "CG", "CZ", "VA", "FM", "MD", "PS", "KR", "TW", "TZ", "TL", "GB"}

// judgeOpenAIResponse 依据「HTTP 状态码 + 响应体特征」判定单次探测结果。
// 优先真实 API（/v1/models 无鉴权 401 = 未封、403 = 封禁）；网页类 200+HTML 特征 = 可访问。
func judgeOpenAIResponse(status int, body []byte, url string) openAIVerdict {
	host := strings.ToLower(url)
	lower := bytes.ToLower(body)
	switch {
	case strings.Contains(host, "api.openai.com"):
		// OpenAI API 未鉴权访问 /v1/models：
		//   401 = 到达 OpenAI 且要求鉴权（IP 未被封）
		//   403 = IP 被封禁 / 地区不支持
		switch status {
		case http.StatusUnauthorized: // 401
			return verdictUnlocked
		case http.StatusForbidden: // 403
			return verdictBlocked
		default:
			return verdictUnknown
		}
	case strings.Contains(host, "chat.openai.com"), strings.Contains(host, "platform.openai.com"):
		switch {
		case status == http.StatusOK && (bytes.Contains(lower, []byte("<!doctype html")) || bytes.Contains(lower, []byte("<html"))):
			return verdictUnlocked
		case status == http.StatusForbidden:
			// OpenAI 封禁响应为 text/plain（如 "Your access was terminated"）；
			// Cloudflare 拦截页为 HTML。两者对 ChatGPT 判定均不可用。
			return verdictBlocked
		default:
			return verdictUnknown
		}
	default:
		return verdictUnknown
	}
}

// Get openai
func testOpenai(p proxy.Proxy) (ok bool, err error) {
	pmap := make(map[string]any)
	err = json.Unmarshal([]byte(p.String()), &pmap)
	if err != nil {
		return false, fmt.Errorf("解析代理配置失败: %w", err)
	}

	pmap["port"] = int(pmap["port"].(float64))
	if p.TypeName() == "vmess" {
		pmap["alterId"] = int(pmap["alterId"].(float64))
		if network, ok := pmap["network"]; ok && network.(string) == "h2" {
			return false, errors.New("暂不支持 h2 协议测试")
		}
	}

	clashProxy, err := adapter.ParseProxy(pmap)
	if err != nil {
		return false, fmt.Errorf("创建代理客户端失败: %w", err)
	}

	// 智能重试机制
	var lastErr error
	for retry := 0; retry <= openAIMaxRetries; retry++ {
		// 自适应超时：首次使用默认超时，之后递增50%
		timeout := openAITestTimeout
		if retry > 0 {
			timeout = time.Duration(float64(timeout) * 1.5)
		}

		// 遍历所有测试URL，任一明确判定即结束
		for _, testURL := range openAITestURLs {
			b, status, err := HTTPGetBodyStatusViaProxyWithTime(clashProxy, testURL, timeout)
			if err != nil {
				lastErr = err
				continue
			}

			switch judgeOpenAIResponse(status, b, testURL) {
			case verdictUnlocked:
				return true, nil
			case verdictBlocked:
				return false, errors.New("IP 已被 OpenAI 封禁或所在地区不支持")
			}
		}

		// 主判据无法定论（响应格式异常/重定向）时，回退 Cloudflare trace 的 loc 国家白名单
		unlocked, decided, _ := openaiTraceFallback(clashProxy, timeout)
		if decided {
			if unlocked {
				return true, nil
			}
			return false, errors.New("当前 IP 所在地区不支持访问 OpenAI")
		}

		// 如果是最后一次重试，返回错误
		if retry == openAIMaxRetries {
			if lastErr != nil {
				return false, fmt.Errorf("OpenAI 连接测试失败: %w", lastErr)
			}
			return false, errors.New("无法判定 OpenAI 可用性")
		}

		// 非最后一次重试，等待一段时间后继续
		time.Sleep(time.Second * time.Duration(retry+1))
	}

	return false, lastErr
}

// openaiTraceFallback 兜底：请求 cdn-cgi/trace 解析 loc=，国家在支持列表即视为可用。
// 返回 (unlocked, decided, err)：decided=false 表示 trace 不可用/无法判定。
func openaiTraceFallback(clashProxy C.Proxy, timeout time.Duration) (bool, bool, error) {
	trace, err := HTTPGetBodyViaProxyWithTime(clashProxy, "https://chat.openai.com/cdn-cgi/trace", timeout)
	if err != nil {
		return false, false, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(trace))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "loc=") {
			loc := strings.TrimPrefix(line, "loc=")
			return slices.Contains(SupportCountry, loc), true, nil
		}
	}
	return false, false, errors.New("trace 响应缺少 loc 字段")
}
