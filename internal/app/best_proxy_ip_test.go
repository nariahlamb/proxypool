package app

import "testing"

// TestParseSubIpSubLine 验证 sub_ip_list_url 明文订阅行的解析：
// 行格式 "ip:port#Country"，端口必须在白名单内，host 必须是合法 IP
func TestParseSubIpSubLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string // 期望返回的 host:port；空串表示应被丢弃
		ok   bool
	}{
		{"标准行(带国家注释)", "104.17.212.191:443#US 🇺🇸", "104.17.212.191:443", true},
		{"标准行(无注释)", "104.25.0.8:443", "104.25.0.8:443", true},
		{"白名单端口8443", "104.18.81.19:8443#US 🇺🇸", "104.18.81.19:8443", true},
		{"白名单端口2053", "158.180.69.78:2053#KR 🇰🇷", "158.180.69.78:2053", true},
		{"白名单端口2083", "1.1.1.1:2083#SG", "1.1.1.1:2083", true},
		{"白名单端口2087", "1.1.1.1:2087", "1.1.1.1:2087", true},
		{"白名单端口2096", "1.1.1.1:2096#US", "1.1.1.1:2096", true},
		{"头注释行", "# 295 bestips updated at 2026-08-01 20:47", "", false},
		{"空行", "", "", false},
		{"纯空白行", "   \t  ", "", false},
		{"非白名单端口80", "1.2.3.4:80#US", "", false},
		{"非白名单端口4430", "1.2.3.4:4430", "", false},
		{"非法host(域名)", "example.com:443", "", false},
		{"缺少端口(默认补443)", "1.2.3.4#US", "1.2.3.4:443", true},
		{"非法host纯域名无端口", "example.com", "", false},
		{"多余冒号非法行", "1.2.3.4:443:extra", "", false},
		{"IPv6白名单端口", "[2606:4700::1111]:443#US", "[2606:4700::1111]:443", true},
		{"行尾CRLF", "104.17.212.191:8443#US\r", "104.17.212.191:8443", true},
		{"首尾空格", "  104.17.212.191:443#US  ", "104.17.212.191:443", true},
		{"纯IP行(ipdb格式,带#)", "47.57.245.232#", "47.57.245.232:443", true},
		{"纯IP行(无#)", "8.212.65.162", "8.212.65.162:443", true},
		{"纯IP行(带国家注释)", "158.180.75.201#KR", "158.180.75.201:443", true},
		{"裸IPv6无端口", "2606:4700::1111", "[2606:4700::1111]:443", true},
		{"裸IPv6带#", "2606:4700::1111#US", "[2606:4700::1111]:443", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseSubIpSubLine(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (got addr %q)", ok, tc.ok, got)
			}
			if got != tc.want {
				t.Fatalf("addr = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBestIpPortWhitelist 验证端口白名单覆盖全部 6 个允许端口
func TestBestIpPortWhitelist(t *testing.T) {
	allowed := []string{"443", "2053", "2083", "2087", "2096", "8443"}
	for _, p := range allowed {
		if _, ok := bestIpPortWhitelist[p]; !ok {
			t.Errorf("port %s should be in whitelist", p)
		}
	}
	disallowed := []string{"80", "8080", "4430", "2082", "2095", "8444", "0", "-1", "abc"}
	for _, p := range disallowed {
		if _, ok := bestIpPortWhitelist[p]; ok {
			t.Errorf("port %s should NOT be in whitelist", p)
		}
	}
}

// TestParsePlainSubLine 验证明文 "host:port#注释" 订阅行解析（bestcf.pages.dev 域名列表格式）
func TestParsePlainSubLine(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		host     string
		port     string
		fragment string
		ok       bool
	}{
		{"域名带中文注释", "polestar.com:443#亚洲域名 | 中国极星汽车", "polestar.com", "443", "亚洲域名 | 中国极星汽车", true},
		{"IP带注释", "162.159.198.1:443#亚洲域名 | 优选网", "162.159.198.1", "443", "亚洲域名 | 优选网", true},
		{"域名无注释", "deepin.org:443", "deepin.org", "443", "", true},
		{"无端口带注释", "70mai.store#亚洲域名 | 中国70迈", "70mai.store", "443", "亚洲域名 | 中国70迈", true},
		{"纯域名无端口", "52pojie.org", "52pojie.org", "443", "", true},
		{"多级域名", "www.nestle.com.cn:443#中国雀巢", "www.nestle.com.cn", "443", "中国雀巢", true},
		{"IPv6带方括号", "[2606:4700::1111]:443#US", "2606:4700::1111", "443", "US", true},
		{"头注释行", "# 84 bestcf domains", "", "", "", false},
		{"空行", "", "", "", "", false},
		{"纯空白行", "   ", "", "", "", false},
		{"非白名单端口", "1.2.3.4:80#US", "", "", "", false},
		{"裸IPv6无方括号(歧义,视为IPv6)", "2606:4700::1111:443#US", "2606:4700::1111:443", "443", "US", true},
		{"过多冒号非法行", "a:b:c:d:e:f:1:2:3:4#US", "", "", "", false},
		{"行尾CRLF", "deepin.org:443#深度系统\r", "deepin.org", "443", "深度系统", true},
		{"首尾空格", "  polestar.com:443#亚洲  ", "polestar.com", "443", "亚洲", true},
		{"注释内多个#", "a.com:443#x#y", "a.com", "443", "x#y", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host, port, fragment, ok := parsePlainSubLine(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (got %s:%s)", ok, tc.ok, host, port)
			}
			if host != tc.host || port != tc.port || fragment != tc.fragment {
				t.Fatalf("got (%q,%q,%q), want (%q,%q,%q)", host, port, fragment, tc.host, tc.port, tc.fragment)
			}
		})
	}
}
