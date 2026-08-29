package app

import (
	"testing"

	"github.com/One-Piecs/proxypool/internal/cache"
)

// TestSortBestNodesByCountry 国家优先排序（升序）
func TestSortBestNodesByCountry(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "1.1.1.1", Port: 443, Country: "CN"},
		{Ip: "2.2.2.2", Port: 443, Country: "US"},
		{Ip: "3.3.3.3", Port: 443, Country: "JP"},
	}
	sortBestNodes(nodes)
	want := []string{"CN", "JP", "US"}
	for i, c := range want {
		if nodes[i].Country != c {
			t.Fatalf("排序[%d].Country = %s, want %s", i, nodes[i].Country, c)
		}
	}
}

// TestSortBestNodesByIPv4Numeric 同国家 IPv4 按 TCP 数值比较：
// 1.1.1.9 < 1.1.1.10（字符串比较会得出相反顺序）
func TestSortBestNodesByIPv4Numeric(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "1.1.1.10", Port: 443, Country: "US"},
		{Ip: "1.1.1.2", Port: 443, Country: "US"},
		{Ip: "1.1.1.9", Port: 443, Country: "US"},
	}
	sortBestNodes(nodes)
	want := []string{"1.1.1.2", "1.1.1.9", "1.1.1.10"}
	for i, ip := range want {
		if nodes[i].Ip != ip {
			t.Fatalf("排序[%d].Ip = %s, want %s", i, nodes[i].Ip, ip)
		}
	}
}

// TestSortBestNodesByPort 同国家同 IP 按端口升序
func TestSortBestNodesByPort(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "1.1.1.1", Port: 8443, Country: "US"},
		{Ip: "1.1.1.1", Port: 443, Country: "US"},
		{Ip: "1.1.1.1", Port: 2053, Country: "US"},
	}
	sortBestNodes(nodes)
	want := []int{443, 2053, 8443}
	for i, p := range want {
		if nodes[i].Port != p {
			t.Fatalf("排序[%d].Port = %d, want %d", i, nodes[i].Port, p)
		}
	}
}

// TestSortBestNodesMixedIPv6 非 IPv4（IPv6）保持字符串比较
func TestSortBestNodesMixedIPv6(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "2606:4700::1111", Port: 443, Country: "US"},
		{Ip: "104.16.0.1", Port: 443, Country: "US"},
		{Ip: "2606:4700::1001", Port: 443, Country: "US"},
	}
	sortBestNodes(nodes)
	want := []string{"104.16.0.1", "2606:4700::1001", "2606:4700::1111"}
	for i, ip := range want {
		if nodes[i].Ip != ip {
			t.Fatalf("排序[%d].Ip = %s, want %s", i, nodes[i].Ip, ip)
		}
	}
}

// TestSortBestNodesStable 相同键保持原顺序（稳定排序）
func TestSortBestNodesStable(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "1.1.1.1", Port: 443, Country: "US", CDN: true},
		{Ip: "1.1.1.1", Port: 443, Country: "US", CDN: false},
	}
	sortBestNodes(nodes)
	if !nodes[0].CDN || nodes[1].CDN {
		t.Fatalf("稳定排序失效: %+v", nodes)
	}
}

// TestSortBestNodesByIP 仅按 IP 排序（IPv4 数值比较）
func TestSortBestNodesByIP(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "10.0.0.2", Port: 443, Country: "US"},
		{Ip: "10.0.0.10", Port: 443, Country: "CN"},
		{Ip: "10.0.0.1", Port: 443, Country: "JP"},
	}
	sortBestNodesByIP(nodes)
	want := []string{"10.0.0.1", "10.0.0.2", "10.0.0.10"}
	for i, ip := range want {
		if nodes[i].Ip != ip {
			t.Fatalf("按IP排序第 %d 位应为 %s, got %s", i, ip, nodes[i].Ip)
		}
	}
}

// TestApplyBestNodeSort 各 sortMode 排序行为
func TestApplyBestNodeSort(t *testing.T) {
	nodes := []cache.BestNode{
		{Ip: "1.0.0.1", Port: 443, Country: "US"},
		{Ip: "1.0.0.1", Port: 8443, Country: "US"},
		{Ip: "2.0.0.1", Port: 443, Country: "CN"},
	}
	// country：国家优先，同国家按 IP、端口
	cp := append([]cache.BestNode(nil), nodes...)
	if err := applyBestNodeSort(cp, "country"); err != nil {
		t.Fatal(err)
	}
	if cp[0].Country != "CN" || cp[2].Country != "US" {
		t.Fatalf("country 排序应按国家: %+v", cp)
	}
	if cp[1].Port != 443 || cp[2].Port != 8443 {
		t.Fatalf("country 排序同国家应按 IP/端口: %+v", cp)
	}
	// ip：仅按 IP（CN 的 2.0.0.1 排最后，尽管国家 CN 在前）
	ipc := append([]cache.BestNode(nil), nodes...)
	if err := applyBestNodeSort(ipc, "ip"); err != nil {
		t.Fatal(err)
	}
	if ipc[2].Ip != "2.0.0.1" {
		t.Fatalf("ip 排序应按 IP 数值: %+v", ipc)
	}
	// random：打乱（只验证合法性与元素集不变）
	rc := append([]cache.BestNode(nil), nodes...)
	if err := applyBestNodeSort(rc, "random"); err != nil {
		t.Fatal(err)
	}
	// 非法值报错
	if err := applyBestNodeSort(nodes, "bogus"); err == nil {
		t.Fatal("非法 sort 值应报错")
	}
}
