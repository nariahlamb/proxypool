package proxy

import (
	"strings"
	"testing"

	"github.com/metacubex/mihomo/adapter"
	C "github.com/metacubex/mihomo/constant"
)

// TestAnyTLSLinkRoundTrip 验证 anytls 链接解析与生成往返
func TestAnyTLSLinkRoundTrip(t *testing.T) {
	link := "anytls://pass123@example.com:443?sni=cdn.example.com&alpn=h2%2Chttp%2F1.1#anytls-01"
	a, err := ParseAnyTLSLink(link)
	if err != nil {
		t.Fatal(err)
	}
	if a.TypeName() != "anytls" {
		t.Errorf("type = %q", a.TypeName())
	}
	if a.Password != "pass123" || a.Server != "example.com" || a.Port != 443 {
		t.Errorf("base mismatch: %+v", a.Base)
	}
	if a.SNI != "cdn.example.com" {
		t.Errorf("sni = %q", a.SNI)
	}
	if len(a.ALPN) != 2 || a.ALPN[0] != "h2" {
		t.Errorf("alpn = %v", a.ALPN)
	}

	regen := a.Link()
	a2, err := ParseAnyTLSLink(regen)
	if err != nil {
		t.Fatalf("re-parse failed: %v (%s)", err, regen)
	}
	if a2.Password != a.Password || a2.Server != a.Server || a2.Port != a.Port ||
		a2.SNI != a.SNI || len(a2.ALPN) != len(a.ALPN) {
		t.Errorf("round-trip mismatch: %+v vs %+v", a2, a)
	}
}

// TestAnyTLSParse 验证基本解析
func TestAnyTLSParse(t *testing.T) {
	a, err := ParseAnyTLSLink("anytls://pw@1.2.3.4:8443#node")
	if err != nil {
		t.Fatal(err)
	}
	if a.Server != "1.2.3.4" || a.Port != 8443 || a.Password != "pw" || a.Name != "node" {
		t.Errorf("parse mismatch: %+v", a)
	}
	if _, err := ParseAnyTLSLink("anytls://bad"); err == nil {
		t.Error("expected error for invalid link")
	}
}

// TestAnyTLSOutput 验证各端输出
func TestAnyTLSOutput(t *testing.T) {
	a := &AnyTLS{
		Base:           Base{Name: "AT-01", Server: "example.com", Port: 443, Type: "anytls", UDP: true},
		Password:       "secret",
		SNI:            "cdn.example.com",
		ALPN:           []string{"h2", "http/1.1"},
		SkipCertVerify: true,
	}
	if s := a.ToClash(); !strings.Contains(s, `"type":"anytls"`) || !strings.Contains(s, `"password":"secret"`) {
		t.Errorf("ToClash: %s", s)
	}
	if s := a.ToLoon(); !strings.Contains(s, "anytls") || !strings.Contains(s, "AT-01") {
		t.Errorf("ToLoon: %s", s)
	}
	if s := a.ToQuanX(); !strings.HasPrefix(s, "anytls=") || !strings.Contains(s, "password=secret") || !strings.Contains(s, "over-tls=true") {
		t.Errorf("ToQuanX: %s", s)
	}
	// Surge（iOS 5.17.0+ / Mac 6.4.3+ 支持 AnyTLS v2）
	s := a.ToSurge()
	if !strings.HasPrefix(s, "AT-01 = anytls, example.com, 443, password=secret") {
		t.Errorf("ToSurge prefix: %s", s)
	}
	if !strings.Contains(s, "sni=cdn.example.com") || !strings.Contains(s, "alpn=h2,http/1.1") || !strings.Contains(s, "skip-cert-verify=true") {
		t.Errorf("ToSurge params: %s", s)
	}

	// 无 sni 时回退为 server
	a2 := &AnyTLS{Base: Base{Name: "AT-02", Server: "1.2.3.4", Port: 443, Type: "anytls"}, Password: "pw"}
	if s2 := a2.ToSurge(); !strings.Contains(s2, "sni=1.2.3.4") {
		t.Errorf("ToSurge sni fallback: %s", s2)
	}
}

// TestAnyTLSMihomo 验证 mihomo 能解析 anytls
func TestAnyTLSMihomo(t *testing.T) {
	a := &AnyTLS{
		Base:     Base{Name: "at1", Server: "example.com", Port: 443, Type: "anytls"},
		Password: "secret",
		SNI:      "cdn.example.com",
		ALPN:     []string{"h2", "http/1.1"},
	}
	m := ToClashMap(a)
	if m == nil {
		t.Fatal("ToClashMap returned nil")
	}
	cp, err := adapter.ParseProxy(m)
	if err != nil {
		t.Fatalf("mihomo ParseProxy failed: %v", err)
	}
	if cp.Type() != C.AnyTLS {
		t.Errorf("type = %v, want AnyTLS", cp.Type())
	}
}

// TestGrepAnyTLSLinkFromString 验证链接抓取
func TestGrepAnyTLSLinkFromString(t *testing.T) {
	text := "text anytls://pw1@a.com:443?sni=x.com#n1 trailing anytls://pw2@b.com:443#n2"
	links := GrepAnyTLSLinkFromString(text)
	if len(links) != 2 {
		t.Fatalf("found %d links, want 2: %v", len(links), links)
	}
}
