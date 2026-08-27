package healthcheck

import (
	"testing"

	"github.com/One-Piecs/proxypool/pkg/proxy"
)

// newTestSSProxy 构造一个 ss 代理（IP 直连，避免 DNS；GeoIP 库未加载时降级为默认国家）
func newTestSSProxy(t *testing.T, b64 string) proxy.Proxy {
	t.Helper()
	p, err := proxy.ParseProxyFromLink("ss://" + b64)
	if err != nil {
		t.Fatalf("ParseProxyFromLink: %v", err)
	}
	return p
}

// TestSortProxiesBySpeed 验证排序规则：
// 1) 有测速记录的排前、无记录的排后；2) 有记录的按速度降序；3) 无记录保持原顺序（稳定排序）
func TestSortProxiesBySpeed(t *testing.T) {
	p1 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczFAMS4xLjEuMTo4Mzg4") // speed 100
	p2 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczJAMS4xLjEuMjo4Mzg4") // speed 50
	p3 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczNAMS4xLjEuMzo4Mzg4") // 无记录
	p4 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczRAMS4xLjEuNDo4Mzg4") // 无记录

	psList := StatList{
		{Id: p1.Identifier(), Speed: 100},
		{Id: p2.Identifier(), Speed: 50},
	}
	in := []proxy.Proxy{p1, p2, p3, p4}
	got := psList.SortProxiesBySpeed(in)

	// 排序为原地操作，返回同一切片
	if &got[0] != &in[0] {
		t.Errorf("SortProxiesBySpeed 应返回原切片")
	}
	want := []string{p1.Identifier(), p2.Identifier(), p3.Identifier(), p4.Identifier()}
	for i, id := range want {
		if got[i].Identifier() != id {
			t.Errorf("位置[%d] = %s, want %s（全序 %v）", i, got[i].Identifier(), id, want)
		}
	}
}

// TestSortProxiesBySpeedOnlyOneHasSpeed 有记录与无记录混合时，无记录保持原相对顺序
func TestSortProxiesBySpeedOnlyOneHasSpeed(t *testing.T) {
	p1 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczFAMS4xLjEuMTo4Mzg4") // speed 100
	p2 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczJAMS4xLjEuMjo4Mzg4") // 无记录
	p3 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczNAMS4xLjEuMzo4Mzg4") // 无记录

	psList := StatList{{Id: p1.Identifier(), Speed: 100}}
	got := psList.SortProxiesBySpeed([]proxy.Proxy{p1, p2, p3})

	want := []string{p1.Identifier(), p2.Identifier(), p3.Identifier()}
	for i, id := range want {
		if got[i].Identifier() != id {
			t.Errorf("位置[%d] = %s, want %s", i, got[i].Identifier(), id)
		}
	}
}

// TestSortProxiesBySpeedSpeedDesc 速度降序
func TestSortProxiesBySpeedSpeedDesc(t *testing.T) {
	p1 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczFAMS4xLjEuMTo4Mzg4") // speed 10
	p2 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczJAMS4xLjEuMjo4Mzg4") // speed 200
	p3 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczNAMS4xLjEuMzo4Mzg4") // speed 50

	psList := StatList{
		{Id: p1.Identifier(), Speed: 10},
		{Id: p2.Identifier(), Speed: 200},
		{Id: p3.Identifier(), Speed: 50},
	}
	got := psList.SortProxiesBySpeed([]proxy.Proxy{p1, p2, p3})

	want := []string{p2.Identifier(), p3.Identifier(), p1.Identifier()}
	for i, id := range want {
		if got[i].Identifier() != id {
			t.Errorf("位置[%d] = %s, want %s", i, got[i].Identifier(), id)
		}
	}
}

// TestSortProxiesBySpeedEdgeCases 边界：nil/空/单元素原样返回
func TestSortProxiesBySpeedEdgeCases(t *testing.T) {
	var nilSlice []proxy.Proxy
	if got := (StatList{}).SortProxiesBySpeed(nilSlice); got != nil {
		t.Errorf("nil 输入应原样返回 nil, got %v", got)
	}
	empty := []proxy.Proxy{}
	if got := (StatList{}).SortProxiesBySpeed(empty); len(got) != 0 {
		t.Errorf("空输入应原样返回空, got %v", got)
	}
	p1 := newTestSSProxy(t, "YWVzLTI1Ni1jZmI6cGFzczFAMS4xLjEuMTo4Mzg4")
	single := []proxy.Proxy{p1}
	if got := (StatList{}).SortProxiesBySpeed(single); len(got) != 1 {
		t.Errorf("单元素应原样返回, got %v", got)
	}
}
