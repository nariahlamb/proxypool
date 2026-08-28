package app

import (
	"os"
	"strings"
	"testing"

	"github.com/One-Piecs/proxypool/config"
	appcache "github.com/One-Piecs/proxypool/internal/cache"
)

// parseTestConfig 写入临时 yaml 并加载到全局配置
func parseTestConfig(cfgYAML string) error {
	path, err := os.CreateTemp("", "pp-test-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(path.Name())
	if _, err := path.WriteString(cfgYAML); err != nil {
		return err
	}
	path.Close()
	return config.Parse(path.Name())
}

// TestSubNiceProxyIpIPV6Modes 集成验证三态输出：
// 默认(0) IPv4+IPv6 都输出；ipv6=true(1) 仅 IPv6；ipv6=false(2) 仅 IPv4。
func TestSubNiceProxyIpIPV6Modes(t *testing.T) {
	// 配置 proxy_info 提供 JP vless 模板
	cfgYAML := "proxy_info:\n  JP:\n    vless:\n      host: \"example.com\"\n      uuid: \"u\"\n      path: \"/p\"\n"
	if err := parseTestConfig(cfgYAML); err != nil {
		t.Fatalf("config parse: %v", err)
	}
	appcache.SetBestNodeList("bestNode", []appcache.BestNode{
		{Ip: "1.2.3.4", Port: 443, Country: "JP"},       // IPv4
		{Ip: "2001:db8::1", Port: 443, Country: "JP"},   // IPv6
	})

	out, err := SubNiceProxyIp("clashVless", "JP", "", 0, false, 0, "")
	if err != nil {
		t.Fatalf("mode=0: %v", err)
	}
	if !strings.Contains(out, "1.2.3.4") || !strings.Contains(out, "[2001:db8::1]") {
		t.Errorf("mode=0 应同时含 IPv4 与 [IPv6]:\n%s", out)
	}

	out, err = SubNiceProxyIp("clashVless", "JP", "", 0, false, 1, "")
	if err != nil {
		t.Fatalf("mode=1: %v", err)
	}
	if !strings.Contains(out, "[2001:db8::1]") || strings.Contains(out, "1.2.3.4") {
		t.Errorf("mode=1 应仅含 IPv6:\n%s", out)
	}

	out, err = SubNiceProxyIp("clashVless", "JP", "", 0, false, 2, "")
	if err != nil {
		t.Fatalf("mode=2: %v", err)
	}
	if !strings.Contains(out, "1.2.3.4") || strings.Contains(out, "[2001:db8::1]") {
		t.Errorf("mode=2 应仅含 IPv4:\n%s", out)
	}
}
