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
