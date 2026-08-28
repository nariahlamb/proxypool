package database

import (
	"testing"

	"github.com/One-Piecs/proxypool/internal/cache"
	"github.com/One-Piecs/proxypool/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupBestNodeDB 初始化内存 sqlite 并迁移 best 节点表
func setupBestNodeDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&BestNodeDB{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	DB = db
	t.Cleanup(func() { DB = nil })
}

// TestSaveLoadBestNodes 保存 → 加载往返（含探测标记字段）
func TestSaveLoadBestNodes(t *testing.T) {
	log.SetLevel(log.ERROR)
	setupBestNodeDB(t)

	nodes := []cache.BestNode{
		{Ip: "1.1.1.1", Port: 443, Country: "US", CDN: true, AnyTLS: true, Healthy: true},
		{Ip: "1.1.1.2", Port: 8443, Country: "JP", AnyTLS: false, Healthy: true},
		{Ip: "2606:4700::1", Port: 2053, Country: "HK", AnyTLS: true, Healthy: false},
	}
	if err := SaveBestNodes(nodes); err != nil {
		t.Fatalf("save: %v", err)
	}

	got := LoadBestNodes()
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// 顺序应与保存一致
	want := nodes
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("节点[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestSaveBestNodesOverwrite 全量覆盖：第二次保存替换旧数据（表不增长）
func TestSaveBestNodesOverwrite(t *testing.T) {
	log.SetLevel(log.ERROR)
	setupBestNodeDB(t)

	if err := SaveBestNodes([]cache.BestNode{
		{Ip: "1.1.1.1", Port: 443, Country: "US"},
		{Ip: "1.1.1.2", Port: 443, Country: "JP"},
	}); err != nil {
		t.Fatal(err)
	}
	// 第二次只保存 1 个 → 应只剩 1 个
	if err := SaveBestNodes([]cache.BestNode{
		{Ip: "9.9.9.9", Port: 443, Country: "US"},
	}); err != nil {
		t.Fatal(err)
	}
	got := LoadBestNodes()
	if len(got) != 1 || got[0].Ip != "9.9.9.9" {
		t.Fatalf("覆盖后 = %+v, want 仅 9.9.9.9", got)
	}
}

// TestSaveBestNodesEmpty 保存空列表 → 清空表
func TestSaveBestNodesEmpty(t *testing.T) {
	log.SetLevel(log.ERROR)
	setupBestNodeDB(t)

	SaveBestNodes([]cache.BestNode{{Ip: "1.1.1.1", Port: 443}})
	if err := SaveBestNodes(nil); err != nil {
		t.Fatalf("save empty: %v", err)
	}
	if got := LoadBestNodes(); len(got) != 0 {
		t.Fatalf("清空后应无节点, got %d", len(got))
	}
}
