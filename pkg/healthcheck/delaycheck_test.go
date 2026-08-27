package healthcheck

import (
	"strings"
	"testing"
)

// TestDefaultTestURLs 验证默认测试地址列表的约束：
// 全部为 204 端点（不含 200 类页面）、不含 gstatic、国内可达端点靠前
func TestDefaultTestURLs(t *testing.T) {
	urls := getTestURLs()
	if len(urls) < maxTestURLs {
		t.Fatalf("默认列表长度 %d 小于每轮尝试数 %d", len(urls), maxTestURLs)
	}

	for _, u := range urls {
		if strings.Contains(u, "gstatic.com") {
			t.Errorf("默认列表不应含 gstatic（VPS 常被限流）: %s", u)
		}
		// 全部应为 generate_204 类端点（200 类页面会浪费请求）
		if !strings.Contains(u, "generate_204") {
			t.Errorf("默认列表应全为 204 端点: %s", u)
		}
	}

	// 前 maxTestURLs 个内应有国内可达端点（cp.cloudflare / miui）
	first := strings.Join(urls[:maxTestURLs], " ")
	if !strings.Contains(first, "cp.cloudflare.com") {
		t.Errorf("前 %d 个应含 cp.cloudflare.com: %v", maxTestURLs, urls[:maxTestURLs])
	}
}

// TestSetTestURLs 验证配置覆盖与恢复默认
func TestSetTestURLs(t *testing.T) {
	// 覆盖
	custom := []string{"https://a.com/204", "https://b.com/204"}
	SetTestURLs(custom)
	got := getTestURLs()
	if len(got) != 2 || got[0] != custom[0] || got[1] != custom[1] {
		t.Fatalf("SetTestURLs 后 = %v, want %v", got, custom)
	}

	// 空列表恢复默认
	SetTestURLs(nil)
	got = getTestURLs()
	if !strings.Contains(got[0], "cp.cloudflare.com") {
		t.Errorf("恢复默认后首个应为 cp.cloudflare.com: %v", got)
	}

	// 覆盖后不应影响原默认切片（防别名共享）
	SetTestURLs(custom)
	got2 := getTestURLs()
	got2[0] = "mutated"
	if getTestURLs()[0] != custom[0] {
		t.Error("getTestURLs 应返回副本")
	}
	SetTestURLs(nil) // 复原，避免影响其他测试
}
