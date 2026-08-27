package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/One-Piecs/proxypool/config"
)

// TestGeneratorKeyAnytls 验证 5 个客户端的 anytls 生成器 key 映射
func TestGeneratorKeyAnytls(t *testing.T) {
	cases := []struct {
		f    Format
		want string
	}{
		{Format{Surge: true, Anytls: true}, "surge_anytls"},
		{Format{Clash: true, Anytls: true}, "clash_anytls"},
		{Format{QuanX: true, Anytls: true}, "quanx_anytls"},
		{Format{Loon: true, Anytls: true}, "loon_anytls"},
		{Format{V2rayn: true, Anytls: true}, "v2rayn_anytls"},
	}
	for _, tc := range cases {
		if got := generatorKey(tc.f); got != tc.want {
			t.Errorf("generatorKey(%+v) = %q, want %q", tc.f, got, tc.want)
		}
	}
}

// genTestProxyInfo 构造含 anytls 的 proxy_info
func genTestProxyInfo() config.ProxyInfo {
	return config.ProxyInfo{
		"JP": {
			"anytls": {
				"password":         "anytls-pass",
				"host":             "1.top",
				"alpn":             "h2,http/1.1",
				"skip_cert_verify": true,
			},
		},
	}
}

// TestGenAnytlsUrls 验证 5 个客户端 anytls 生成器输出（6 参数版）
func TestGenAnytlsUrls(t *testing.T) {
	pi := genTestProxyInfo()
	country, ip, port := "🇯🇵", "1.2.3.4", 443

	t.Run("surge", func(t *testing.T) {
		var buf strings.Builder
		genSurgeAnytlsUrl(&buf, pi, "JP", country, ip, port)
		s := buf.String()
		if !strings.HasPrefix(s, "🇯🇵 1.2.3.4:443 = anytls, 1.2.3.4") {
			t.Errorf("prefix: %s", s)
		}
		if !strings.Contains(s, ", 443, password=anytls-pass, sni=1.top") {
			t.Errorf("params: %s", s)
		}
	})

	t.Run("clash", func(t *testing.T) {
		var buf strings.Builder
		genClashAnytlsUrl(&buf, pi, "JP", country, ip, port)
		s := buf.String()
		if !strings.HasPrefix(s, "  - ") || !strings.Contains(s, `"type":"anytls"`) ||
			!strings.Contains(s, `"server":"1.2.3.4"`) || !strings.Contains(s, `"port":443`) ||
			!strings.Contains(s, `"password":"anytls-pass"`) || !strings.Contains(s, `"sni":"1.top"`) ||
			!strings.Contains(s, `"alpn":["h2","http/1.1"]`) || !strings.Contains(s, `"skip-cert-verify":true`) {
			t.Errorf("clash: %s", s)
		}
	})

	t.Run("quanx", func(t *testing.T) {
		var buf strings.Builder
		genQuanXAnytlsUrl(&buf, pi, "JP", country, ip, port)
		s := buf.String()
		want := "anytls=1.2.3.4:443, password=anytls-pass, over-tls=true, udp-relay=true, tls-host=1.top, tag=🇯🇵 1.2.3.4:443\n"
		if s != want {
			t.Errorf("quanx: got %q, want %q", s, want)
		}
	})

	t.Run("loon", func(t *testing.T) {
		var buf strings.Builder
		genLoonAnytlsUrl(&buf, pi, "JP", country, ip, port)
		s := buf.String()
		want := "🇯🇵 1.2.3.4:443 = anytls, 1.2.3.4, 443, \"anytls-pass\", over-tls=true, tls-name=1.top\n"
		if s != want {
			t.Errorf("loon: got %q, want %q", s, want)
		}
	})

	t.Run("v2rayn", func(t *testing.T) {
		var buf strings.Builder
		genV2raynAnytlsUrl(&buf, pi, "JP", country, ip, port)
		s := buf.String()
		if !strings.HasPrefix(s, "anytls://anytls-pass@1.2.3.4:443?") ||
			!strings.Contains(s, "sni=1.top") || !strings.Contains(s, "alpn=h2%2Chttp%2F1.1") ||
			!strings.Contains(s, "allowInsecure=1") || !strings.Contains(s, "#") {
			t.Errorf("v2rayn: %s", s)
		}
	})

	t.Run("7参数版 nodeName", func(t *testing.T) {
		var buf strings.Builder
		genSurgeAnytlsUrl2(&buf, pi, "JP", country, "亚州优选 1", ip, port)
		s := buf.String()
		if !strings.HasPrefix(s, "🇯🇵 亚州优选 1 = anytls, 1.2.3.4") {
			t.Errorf("surge2: %s", s)
		}
	})
}

// TestCheckFormatAnytls 验证 checkFormat 对 anytls 的识别与配置校验
func TestCheckFormatAnytls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
proxy_info:
  JP:
    anytls:
      host: "1.top"
      password: "anytls-pass"
  CN:
    vmess:
      host: "3.top"
      uuid: "24b566e4-8ef6-4693-b502-26c43ac49fb7"
      path: "/path3"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.Parse(path); err != nil {
		t.Fatalf("config parse: %v", err)
	}

	// surgeAnytls 且 JP 配置了 anytls → 通过
	f, err := checkFormat("surgeAnytls", "JP")
	if err != nil {
		t.Fatalf("checkFormat(surgeAnytls, JP): %v", err)
	}
	if !f.Surge || !f.Anytls {
		t.Errorf("format flags = %+v, want Surge+Anytls", f)
	}

	// 其余客户端 anytls 均通过 checkFormat（生成器缺失时输出为空，但格式合法）
	for _, fm := range []string{"clashAnytls", "quanxAnytls", "loonAnytls", "v2raynAnytls"} {
		if _, err := checkFormat(fm, "JP"); err != nil {
			t.Errorf("checkFormat(%s, JP): %v", fm, err)
		}
	}

	// CN 未配置 anytls → 报错（信息应指出国家与配置方法）
	if _, err := checkFormat("surgeAnytls", "CN"); err == nil {
		t.Error("checkFormat(surgeAnytls, CN) should fail (no anytls node)")
	} else if !strings.Contains(err.Error(), "CN") || !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("错误信息应包含国家名与配置指引: %v", err)
	}
	// 未知国家 → 报错
	if _, err := checkFormat("surgeAnytls", "US"); err == nil {
		t.Error("checkFormat(surgeAnytls, US) should fail (unknown country)")
	}
}
