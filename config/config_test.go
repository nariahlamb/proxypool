package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	writeCfg := func(port string) {
		if err := os.WriteFile(path, []byte("port: \""+port+"\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// 确保 mtime 严格变化（APFS 纳秒精度下本可省略，双保险）
		_ = os.Chtimes(path, time.Now(), time.Now().Add(2*time.Second))
	}

	writeCfg("12345")
	if err := Parse(path); err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if Config().Port != "12345" {
		t.Fatalf("port = %q, want 12345", Config().Port)
	}

	// 未变化：应命中缓存（不重新解析）
	if err := Parse(path); err != nil {
		t.Fatalf("cached parse: %v", err)
	}
	if Config().Port != "12345" {
		t.Fatalf("port = %q, want 12345 (cached)", Config().Port)
	}

	// 文件变化：应重新解析
	writeCfg("99999")
	if err := Parse(path); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if Config().Port != "99999" {
		t.Fatalf("port = %q, want 99999 (re-parsed)", Config().Port)
	}
}

func TestParseInvalidPath(t *testing.T) {
	if err := Parse(filepath.Join(t.TempDir(), "nonexistent.yaml")); err == nil {
		t.Fatal("expected error for nonexistent config file")
	}
}

// TestParseSubIpListUrl 验证 sub_ip_list_url 配置项解析
func TestParseSubIpListUrl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `sub_ip_list_url:
  - "https://raw.githubusercontent.com/LancelotRar/best-cf-ips/main/best-cf-ipv4.txt"
  - https://example.com/list.txt
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Parse(path); err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := Config().SubIpListUrl
	if len(got) != 2 {
		t.Fatalf("SubIpListUrl len = %d, want 2", len(got))
	}
	want := []string{
		"https://raw.githubusercontent.com/LancelotRar/best-cf-ips/main/best-cf-ipv4.txt",
		"https://example.com/list.txt",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SubIpListUrl[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// 未配置时为空
	path2 := filepath.Join(t.TempDir(), "config2.yaml")
	if err := os.WriteFile(path2, []byte("port: \"12580\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Parse(path2); err != nil {
		t.Fatalf("parse2: %v", err)
	}
	if len(Config().SubIpListUrl) != 0 {
		t.Errorf("SubIpListUrl should be empty when not configured, got %v", Config().SubIpListUrl)
	}
}
