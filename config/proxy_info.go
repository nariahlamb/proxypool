package config

import "sort"

type (
	ProxyInfo map[string]ProxyType
	ProxyType map[string]map[string]any
)

// protocolOrder 页面动态生成器中的协议展示顺序
var protocolOrder = []string{"vless", "vmess", "trojan", "anytls"}

// Countries 返回已配置国家的排序列表（供页面落地国家下拉使用）
func (p ProxyInfo) Countries() []string {
	cs := make([]string, 0, len(p))
	for c := range p {
		cs = append(cs, c)
	}
	sort.Strings(cs)
	return cs
}

// CountryProtocols 返回每个国家实际配置的协议列表（按固定顺序过滤）
func (p ProxyInfo) CountryProtocols() map[string][]string {
	out := make(map[string][]string, len(p))
	for c, types := range p {
		var ps []string
		for _, name := range protocolOrder {
			if _, ok := types[name]; ok {
				ps = append(ps, name)
			}
		}
		out[c] = ps
	}
	return out
}
