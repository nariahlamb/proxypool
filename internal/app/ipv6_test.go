package app

import (
	"testing"

	appcache "github.com/One-Piecs/proxypool/internal/cache"
)

func TestFormatNodeHost(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"1.2.3.4", "1.2.3.4"},
		{"2001:db8::1", "[2001:db8::1]"},
		{"2606:4700:4700::1111", "[2606:4700:4700::1111]"},
		{"", ""},
	}
	for _, c := range cases {
		if got := formatNodeHost(c.in); got != c.want {
			t.Errorf("formatNodeHost(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestMatchIPV6Mode 三态过滤语义：
// 默认(0) IPv4+IPv6 都输出；1 仅 IPv6；2 仅 IPv4。
func TestMatchIPV6Mode(t *testing.T) {
	v4 := "1.2.3.4"
	v6 := "2001:db8::1"

	// 默认：都输出
	if !matchIPV6Mode(0, v4) || !matchIPV6Mode(0, v6) {
		t.Error("mode=0 应同时输出 IPv4 与 IPv6")
	}
	// 仅 IPv6
	if matchIPV6Mode(1, v4) || !matchIPV6Mode(1, v6) {
		t.Error("mode=1 应仅输出 IPv6")
	}
	// 仅 IPv4
	if !matchIPV6Mode(2, v4) || matchIPV6Mode(2, v6) {
		t.Error("mode=2 应仅输出 IPv4")
	}
}

func TestIsIPv6(t *testing.T) {
	if IsIPv6("1.2.3.4") {
		t.Error("IsIPv6(1.2.3.4) = true, want false")
	}
	if !IsIPv6("2001:db8::1") {
		t.Error("IsIPv6(2001:db8::1) = false, want true")
	}
}

// TestCountBestV6Healthy 统计健康检查通过的 IPv6 节点数
func TestCountBestV6Healthy(t *testing.T) {
	appcache.SetBestNodeList("bestNode", []appcache.BestNode{
		{Ip: "1.2.3.4", Port: 443, Country: "JP", Healthy: true},       // IPv4 健康
		{Ip: "2001:db8::1", Port: 443, Country: "JP", Healthy: true},   // IPv6 健康
		{Ip: "2001:db8::2", Port: 443, Country: "JP", Healthy: false},  // IPv6 不健康
		{Ip: "2001:db8::3", Port: 443, Country: "JP"},                  // IPv6 无标记（未探测）
	})
	if got := CountBestV6Healthy(); got != 1 {
		t.Errorf("CountBestV6Healthy = %d, want 1", got)
	}
	appcache.SetBestNodeList("bestNode", nil)
	if got := CountBestV6Healthy(); got != 0 {
		t.Errorf("empty list = %d, want 0", got)
	}
}

// TestIsValidHostname 校验 host 合法性：拒绝 HTML/JS 脚本噪音，接受域名/IP
func TestIsValidHostname(t *testing.T) {
	valid := []string{"1.2.3.4", "2606:4700::1", "cf.900501.xyz", "8.889288.xyz", "steep.laibas.top", "a-b.c-d.com", "115155.xyz"}
	invalid := []string{
		"values[key];if(value)parts.push(key+=\"=\"+encodeURIComponent(value))}img.src=parts.join(\"&\")",
		"window.onerror=function(msg){var img=new Image",
		"(function(w,d){if(!w.navigator)return false",
		"Object.keys(p));}this._nativePrototypes[tag]=p",
		"", "localhost", "no-dot", "sp ace.com", "a[b].com",
	}
	for _, h := range valid {
		if !isValidHostname(h) {
			t.Errorf("isValidHostname(%q) = false, want true", h)
		}
	}
	for _, h := range invalid {
		if isValidHostname(h) {
			t.Errorf("isValidHostname(%q) = true, want false", h)
		}
	}
}
