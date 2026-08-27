package app

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"github.com/One-Piecs/proxypool/config"
	"github.com/One-Piecs/proxypool/internal/cache"
	"github.com/One-Piecs/proxypool/log"
	"github.com/One-Piecs/proxypool/pkg/proxy"
	"github.com/gammazero/workerpool"
	"github.com/metacubex/mihomo/adapter"
	C "github.com/metacubex/mihomo/constant"
)

// probeAnyTLSCountry 选择探测用国家凭据：配置指定优先，缺省取 proxy_info 中
// 第一个配置了 anytls 段的国家（map 遍历顺序不定，但任意一个含 anytls 凭据的国家即可，
// 因为标记的是 ip:port 的透传能力，与具体凭据无关）。
func probeAnyTLSCountry(cfg *config.AnyTLSProbeConfig) (string, bool) {
	if cfg != nil && cfg.Country != "" {
		if _, ok := config.Config().ProxyInfo[cfg.Country]["anytls"]; ok {
			return cfg.Country, true
		}
		return "", false
	}
	for country, types := range config.Config().ProxyInfo {
		if _, ok := types["anytls"]; ok {
			return country, true
		}
	}
	return "", false
}

// probeAnyTLSNode 探测单个 ip:port 能否透传 anytls：构造临时 anytls 出站
// （server=候选 ip, port=候选 port, sni=源站域名），mihomo 完整握手
// （TCP + TLS + anytls 协议鉴权），DialContext 成功即代表可转发。
func probeAnyTLSNode(ip string, port int, password, sni string, timeout time.Duration) bool {
	at := &proxy.AnyTLS{
		Base:     proxy.Base{Server: ip, Port: port, Type: "anytls"},
		Password: password,
		SNI:      sni,
		UDP:      true,
	}
	clashProxy, err := adapter.ParseProxy(proxy.ToClashMap(at))
	if err != nil {
		log.Errorln("anytls probe ParseProxy(%s:%d): %v", ip, port, err)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	addr := C.Metadata{Host: "1.1.1.1", DstIP: netip.Addr{}, DstPort: 80}
	conn, err := clashProxy.DialContext(ctx, &addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ProbeAndMarkAnyTLS 并发探测 best 节点列表，返回带 AnyTLS 标记的新切片（不修改入参）。
// 未启用探测或无可探测国家凭据时原样返回。
func ProbeAndMarkAnyTLS(nodes []cache.BestNode) []cache.BestNode {
	cfg := config.Config().AnyTLSProbe
	if cfg == nil || !cfg.Enabled() {
		return nodes
	}
	country, ok := probeAnyTLSCountry(cfg)
	if !ok {
		log.Errorln("anytls probe: no country with anytls credentials configured in proxy_info")
		return nodes
	}
	anytlsInfo := config.Config().ProxyInfo[country]["anytls"]
	password, _ := anytlsInfo["password"].(string)
	sni, _ := anytlsInfo["host"].(string)
	if password == "" || sni == "" {
		log.Errorln("anytls probe: country [%s] anytls missing password/host", country)
		return nodes
	}

	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 20
	}
	timeout := time.Duration(cfg.Timeout) * time.Second
	if cfg.Timeout <= 0 {
		timeout = 5 * time.Second
	}
	if len(nodes) == 0 {
		return nodes
	}

	log.Infoln("anytls probe: testing %d nodes (country=%s, concurrency=%d, timeout=%ds)", len(nodes), country, concurrency, int(timeout/time.Second))

	results := make([]bool, len(nodes))
	wp := workerpool.New(concurrency)
	var wg sync.WaitGroup
	for i, node := range nodes {
		i, node := i, node
		wg.Add(1)
		wp.Submit(func() {
			defer wg.Done()
			results[i] = probeAnyTLSNode(node.Ip, node.Port, password, sni, timeout)
		})
	}
	wg.Wait()
	wp.Stop()

	marked := make([]cache.BestNode, len(nodes))
	count := 0
	for i, node := range nodes {
		node.AnyTLS = results[i]
		marked[i] = node
		if node.AnyTLS {
			count++
		}
	}
	log.Infoln("anytls probe done: %d/%d nodes can relay anytls", count, len(nodes))
	return marked
}

// filterAnyTLSNodes 过滤仅可透传 anytls 的节点。
// 未启用探测时返回空切片（无探测配置 → anytls 导出为空）。
func filterAnyTLSNodes(nodes []cache.BestNode) []cache.BestNode {
	cfg := config.Config().AnyTLSProbe
	if cfg == nil || !cfg.Enabled() {
		log.Warnln("anytls export: probe disabled (missing anytls_probe config), exporting empty list")
		return nil
	}
	filtered := make([]cache.BestNode, 0, len(nodes))
	for _, node := range nodes {
		if node.AnyTLS {
			filtered = append(filtered, node)
		}
	}
	return filtered
}
