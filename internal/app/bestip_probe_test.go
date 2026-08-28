package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/One-Piecs/proxypool/config"
	"github.com/One-Piecs/proxypool/internal/cache"
)

// setupBestIPProbeConfig 构造含 proxy_info(vless) 与 bestip_probe 的临时配置
func setupBestIPProbeConfig(t *testing.T, probeYAML string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
proxy_info:
  JP:
    vless:
      host: "1.top"
      uuid: "24b566e4-8ef6-4693-b502-26c43ac49fb7"
      path: "/path1"
  KR:
    vless:
      host: "2.top"
      uuid: "24b566e4-8ef6-4693-b502-26c43ac49fb7"
      path: "/path2"
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

// TestProbeBestIPCountry 验证探测国家选择：配置指定优先，缺省自动选择第一个含 vless 的国家
func TestProbeBestIPCountry(t *testing.T) {
	setupBestIPProbeConfig(t, "bestip_probe:\n  enable: true\n")

	// 缺省：自动选择（JP/KR 都含 vless，取其一即可，须为配置过的国家）
	country, ok := probeBestIPCountry(config.Config().BestIPProbe)
	if !ok {
		t.Fatal("should find a probe country automatically")
	}
	if _, exists := config.Config().ProxyInfo[country]["vless"]; !exists {
		t.Errorf("auto country %q has no vless config", country)
	}

	// 配置指定：KR
	cfg := &config.BestIPProbeConfig{Country: "KR"}
	if c, ok := probeBestIPCountry(cfg); !ok || c != "KR" {
		t.Errorf("specified country = %q/%v, want KR/true", c, ok)
	}

	// 配置指定但该国家无 vless → 失败
	bad := &config.BestIPProbeConfig{Country: "CN"}
	if _, ok := probeBestIPCountry(bad); ok {
		t.Error("CN has no vless, should fail")
	}

	// 无 bestip_probe 段（nil）→ 缺省自动选择仍可用（探测国家与段是否启用无关）
	if _, ok := probeBestIPCountry(nil); !ok {
		t.Error("nil config should still auto-select country from proxy_info")
	}
}

// TestFilterHealthyNodes 验证优选 IP 健康检查导出过滤：
// 启用探测 → 仅输出标记节点；未启用/无配置 → 导出空
func TestFilterHealthyNodes(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "1.1.1.1", Port: 443, Healthy: false},
		{Ip: "1.1.1.2", Port: 8443, Healthy: true},
		{Ip: "1.1.1.3", Port: 2053, Healthy: true},
	}

	t.Run("启用探测", func(t *testing.T) {
		setupBestIPProbeConfig(t, "bestip_probe:\n  enable: true\n")
		got := filterHealthyNodes(nodes)
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2: %+v", len(got), got)
		}
		for _, n := range got {
			if !n.Healthy {
				t.Errorf("未过滤非标记节点: %+v", n)
			}
		}
	})

	t.Run("显式关闭", func(t *testing.T) {
		setupBestIPProbeConfig(t, "bestip_probe:\n  enable: false\n")
		if got := filterHealthyNodes(nodes); len(got) != 0 {
			t.Errorf("enable=false 应导出空, got %d 节点", len(got))
		}
	})

	t.Run("无配置段", func(t *testing.T) {
		setupBestIPProbeConfig(t, "")
		if got := filterHealthyNodes(nodes); got != nil {
			t.Errorf("无 bestip_probe 段应返回 nil, got %+v", got)
		}
	})
}

// TestCountHealthy 统计健康节点数
func TestCountHealthy(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "1.1.1.1", Healthy: true},
		{Ip: "1.1.1.2", Healthy: false},
		{Ip: "1.1.1.3", Healthy: true},
	}
	if got := countHealthy(nodes); got != 2 {
		t.Errorf("countHealthy = %d, want 2", got)
	}
}

// TestProbeAndMarkNodesNoCredential 合并编排在无可探测凭据时安全返回原列表（不 panic）
func TestProbeAndMarkNodesNoCredential(t *testing.T) {
	// proxy_info 无 vless/anytls 凭据，两个探测都启用 → 各自失败但列表原样返回
	setupBestIPProbeConfig(t, "bestip_probe:\n  enable: true\nsni_probe:\n  enable: true\n")
	// 覆盖 ProxyInfo 为无凭据状态：CN 只有 vmess
	config.Config().ProxyInfo = config.ProxyInfo{
		"CN": {"vmess": map[string]any{"host": "3.top"}},
	}
	nodes := []cache.BestNode{
		{Ip: "1.1.1.1", Port: 443},
		{Ip: "1.1.1.2", Port: 8443},
	}
	got := ProbeAndMarkNodes(nodes)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Healthy || got[1].Healthy {
		t.Error("无 vless 凭据不应标记 Healthy")
	}
}
