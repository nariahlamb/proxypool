package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/One-Piecs/proxypool/config"
	"github.com/One-Piecs/proxypool/internal/cache"
)

// setupProbeConfig 构造含 proxy_info(anytls) 与 sni_probe 的临时配置
func setupProbeConfig(t *testing.T, probeYAML string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
proxy_info:
  JP:
    anytls:
      host: "1.top"
      password: "jp-pass"
  KR:
    anytls:
      host: "2.top"
      password: "kr-pass"
  CN:
    vmess:
      host: "3.top"
      uuid: "24b566e4-8ef6-4693-b502-26c43ac49fb7"
      path: "/path3"
` + probeYAML
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.Parse(path); err != nil {
		t.Fatalf("config parse: %v", err)
	}
}

// TestProbeAnyTLSCountry 验证探测国家选择：配置指定优先，缺省自动选择第一个含 anytls 的国家
func TestProbeAnyTLSCountry(t *testing.T) {
	setupProbeConfig(t, "sni_probe:\n  enable: true\n")

	// 缺省：自动选择（JP/KR 都含 anytls，取其一即可，须为配置过的国家）
	country, ok := probeAnyTLSCountry(config.Config().SniProbe)
	if !ok {
		t.Fatal("should find a probe country automatically")
	}
	if _, exists := config.Config().ProxyInfo[country]["anytls"]; !exists {
		t.Errorf("auto country %q has no anytls config", country)
	}

	// 配置指定：KR
	cfg := &config.SniProbeConfig{Country: "KR"}
	if c, ok := probeAnyTLSCountry(cfg); !ok || c != "KR" {
		t.Errorf("specified country = %q/%v, want KR/true", c, ok)
	}

	// 配置指定但该国家无 anytls → 失败
	bad := &config.SniProbeConfig{Country: "CN"}
	if _, ok := probeAnyTLSCountry(bad); ok {
		t.Error("CN has no anytls, should fail")
	}

	// 无 sni_probe 段（nil）→ 缺省自动选择仍可用（探测国家与段是否启用无关）
	if _, ok := probeAnyTLSCountry(nil); !ok {
		t.Error("nil config should still auto-select country from proxy_info")
	}
}

// TestFilterAnyTLSNodes 验证 anytls 导出过滤：
// 启用探测 → 仅输出标记节点；未启用/无配置 → 导出空
func TestFilterAnyTLSNodes(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "1.1.1.1", Port: 443, AnyTLS: false},
		{Ip: "1.1.1.2", Port: 8443, AnyTLS: true},
		{Ip: "1.1.1.3", Port: 2053, AnyTLS: true},
	}

	t.Run("启用探测", func(t *testing.T) {
		setupProbeConfig(t, "sni_probe:\n  enable: true\n")
		got := filterAnyTLSNodes(nodes)
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2: %+v", len(got), got)
		}
		for _, n := range got {
			if !n.AnyTLS {
				t.Errorf("未过滤非标记节点: %+v", n)
			}
		}
	})

	t.Run("显式关闭", func(t *testing.T) {
		setupProbeConfig(t, "sni_probe:\n  enable: false\n")
		if got := filterAnyTLSNodes(nodes); len(got) != 0 {
			t.Errorf("enable=false 应导出空, got %d 节点", len(got))
		}
	})

	t.Run("无配置段", func(t *testing.T) {
		setupProbeConfig(t, "")
		if got := filterAnyTLSNodes(nodes); len(got) != 0 {
			t.Errorf("无 sni_probe 段应导出空, got %d 节点", len(got))
		}
	})
}
