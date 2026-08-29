package app

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/One-Piecs/proxypool/config"
)

// TestAllClientsIPv6Format 各客户端 IPv6 格式断言（精确匹配 server 字段位置）：
// Surge/Loon/Clash server 字段裸 IPv6（节点名仍带方括号）；
// QuanX（连写 addr:port）与 v2rayN（URL host）需方括号。
func TestAllClientsIPv6Format(t *testing.T) {
	path, err := os.CreateTemp("", "pp-c-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path.Name())
	path.WriteString(`proxy_info:
  JP:
    vless:
      host: "o.laibas.top"
      uuid: "u"
      path: "/p"
    trojan:
      host: "o.laibas.top"
      password: "pw"
      path: "/p"
    anytls:
      host: "o.laibas.top"
      password: "pw"
`)
	path.Close()
	if err := config.Parse(path.Name()); err != nil {
		t.Fatal(err)
	}
	proxyInfo, err := loadProxyInfo()
	if err != nil {
		t.Fatal(err)
	}
	nodes := []string{"2606:4700::1"}

	out := func(f Format) string { return buildNodeOutput(nodes, f, proxyInfo, "JP", 0, 443) }

	// Surge trojan：节点名 [addr]:443，server 裸
	so := out(Format{Surge: true, Trojan: true})
	if !strings.Contains(so, "[2606:4700::1]:443 = trojan") {
		t.Errorf("surge 节点名应 [addr]:443:\n%s", so)
	}
	if strings.Contains(so, "trojan, [2606:4700::1]") {
		t.Errorf("surge server 应裸 IPv6:\n%s", so)
	}
	if !strings.Contains(so, "trojan, 2606:4700::1") {
		t.Errorf("surge server 应为裸 IPv6:\n%s", so)
	}

	// Loon vless：server 裸
	lo := out(Format{Loon: true, Vless: true})
	if !strings.Contains(lo, "= vless, 2606:4700::1, 443") {
		t.Errorf("loon server 应裸 IPv6:\n%s", lo)
	}
	if strings.Contains(lo, "= vless, [2606:4700::1]") {
		t.Errorf("loon server 不应带方括号:\n%s", lo)
	}

	// Clash vless：server 裸
	co := out(Format{Clash: true, Vless: true})
	if !strings.Contains(co, `"server":"2606:4700::1"`) {
		t.Errorf("clash server 应裸 IPv6:\n%s", co)
	}
	if strings.Contains(co, `"server":"[2606:4700::1]"`) {
		t.Errorf("clash server 不应带方括号:\n%s", co)
	}

	// QuanX vless：server 字段裸 IPv6（用户确认 QuanX 不需要方括号）
	qo := out(Format{QuanX: true, Vless: true})
	if !strings.Contains(qo, "vless = 2606:4700::1:443") {
		t.Errorf("quanx server 应裸 IPv6 2606:4700::1:443:\n%s", qo)
	}
	if strings.Contains(qo, "vless = [2606:4700::1]:443") {
		t.Errorf("quanx server 不应带方括号:\n%s", qo)
	}

	// v2rayN vless：URL host 方括号（输出 base64，解码验证）
	vo := out(Format{V2rayn: true, Vless: true})
	dec, err := base64.StdEncoding.DecodeString(strings.TrimSpace(vo))
	if err != nil {
		t.Fatalf("v2rayn base64 解码失败: %v", err)
	}
	if !strings.Contains(string(dec), "@[2606:4700::1]:443") {
		t.Errorf("v2rayn URL host 应 [addr]:443:\n%s", string(dec))
	}

	// anytls：Surge/Clash/Loon server 裸，QuanX 方括号
	if s := out(Format{Surge: true, Anytls: true}); strings.Contains(s, "anytls, [2606:4700::1]") {
		t.Errorf("surge anytls server 应裸:\n%s", s)
	}
	if s := out(Format{Clash: true, Anytls: true}); strings.Contains(s, `"server":"[2606:4700::1]"`) {
		t.Errorf("clash anytls server 应裸:\n%s", s)
	}
	if s := out(Format{Loon: true, Anytls: true}); strings.Contains(s, "anytls, [2606:4700::1]") {
		t.Errorf("loon anytls server 应裸:\n%s", s)
	}
	if s := out(Format{QuanX: true, Anytls: true}); !strings.Contains(s, "anytls=2606:4700::1:443") {
		t.Errorf("quanx anytls server 应裸 IPv6:\n%s", s)
	}
}
