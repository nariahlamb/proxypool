package app

import (
	"os"
	"strings"
	"testing"

	"github.com/One-Piecs/proxypool/config"
)

// TestBuildNodeOutputSurgeIPv6 buildNodeOutput（CfProxyIp 系列）Surge 输出：
// 节点名带方括号 [addr]:port，server 字段为裸 IPv6（逗号分隔字段无歧义）。
func TestBuildNodeOutputSurgeIPv6(t *testing.T) {
	path, err := os.CreateTemp("", "pp-bno-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path.Name())
	path.WriteString(`proxy_info:
  JP:
    trojan:
      host: "o.laibas.top"
      password: "pw"
      path: "/ws"
`)
	path.Close()
	if err := config.Parse(path.Name()); err != nil {
		t.Fatalf("config parse: %v", err)
	}

	f := Format{Surge: true, Trojan: true}
	proxyInfo, err := loadProxyInfo()
	if err != nil {
		t.Fatal(err)
	}

	// IPv4 + IPv6 混合：Surge trojan 行
	out := buildNodeOutput([]string{"1.2.3.4", "2606:4700::1"}, f, proxyInfo, "JP", 0, 443)
	// 节点名（行首）：IPv6 带方括号 [addr]:443
	if !strings.Contains(out, "[2606:4700::1]:443 = trojan") {
		t.Errorf("节点名应为 [2606:4700::1]:443:\n%s", out)
	}
	// server 字段（%-15s 含空格填充）：裸 IPv6（无方括号）
	if !strings.Contains(out, "trojan, 2606:4700::1 ") {
		t.Errorf("server 字段应为裸 IPv6 2606:4700::1:\n%s", out)
	}
	// 不应出现双重方括号 server
	if strings.Contains(out, "trojan, [2606:4700::1]") {
		t.Errorf("server 字段不应带方括号:\n%s", out)
	}
	// IPv4 正常
	if !strings.Contains(out, "trojan, 1.2.3.4 ") {
		t.Errorf("IPv4 server 字段异常:\n%s", out)
	}

	// ipv6=false：仅 IPv4
	out4 := buildNodeOutput([]string{"1.2.3.4", "2606:4700::1"}, f, proxyInfo, "JP", 2, 443)
	if !strings.Contains(out4, "1.2.3.4") || strings.Contains(out4, "2606:4700::1") {
		t.Errorf("ipv6=false 应仅 IPv4:\n%s", out4)
	}

	// ipv6=true：仅 IPv6
	out6 := buildNodeOutput([]string{"1.2.3.4", "2606:4700::1"}, f, proxyInfo, "JP", 1, 443)
	if !strings.Contains(out6, "[2606:4700::1]") || strings.Contains(out6, "1.2.3.4") {
		t.Errorf("ipv6=true 应仅 IPv6:\n%s", out6)
	}
}

// TestFormatNodeHostStripBrackets 方括号补/去互逆
func TestFormatNodeHostStripBrackets(t *testing.T) {
	v6 := "2606:4700::1"
	if formatNodeHost(v6) != "[2606:4700::1]" {
		t.Error("formatNodeHost IPv6 应补方括号")
	}
	if stripBrackets("[2606:4700::1]") != v6 {
		t.Error("stripBrackets 应去掉方括号")
	}
	if stripBrackets("1.2.3.4") != "1.2.3.4" {
		t.Error("stripBrackets IPv4 应原样")
	}
}
