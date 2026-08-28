package app

import "testing"

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

func TestIPv6Filter(t *testing.T) {
	// 双向过滤语义：ipv6=true 仅 IPv6；ipv6=false 仅 IPv4
	ipv4 := "1.2.3.4"
	ipv6 := "2001:db8::1"
	if IsIPv6(ipv4) {
		t.Error("IsIPv6(1.2.3.4) = true, want false")
	}
	if !IsIPv6(ipv6) {
		t.Error("IsIPv6(2001:db8::1) = false, want true")
	}
	// 模拟过滤条件
	if v6 := true; v6 != IsIPv6(ipv6) || v6 == IsIPv6(ipv4) {
		t.Error("ipv6=true 过滤失败")
	}
	if v6 := false; v6 != IsIPv6(ipv4) || v6 == IsIPv6(ipv6) {
		t.Error("ipv6=false 过滤失败")
	}
}
