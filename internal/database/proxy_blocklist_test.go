package database

import (
	"testing"
	"time"

	"github.com/One-Piecs/proxypool/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupBlockListDB 初始化内存 sqlite 并迁移相关表
func setupBlockListDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ProxyBlockList{}, &Proxy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	DB = db
	t.Cleanup(func() { DB = nil })
}

// TestFreezeUnfreeze 冻结 → 查询 → 解冻
func TestFreezeUnfreeze(t *testing.T) {
	log.SetLevel(log.ERROR)
	setupBlockListDB(t)

	id := "1.1.1.1:443"
	if IsFrozen(id) {
		t.Fatal("初始不应冻结")
	}
	if err := FreezeProxy(id); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if !IsFrozen(id) {
		t.Fatal("冻结后应处于冻结态")
	}
	if err := UnfreezeProxy(id); err != nil {
		t.Fatalf("unfreeze: %v", err)
	}
	if IsFrozen(id) {
		t.Fatal("解冻后不应冻结")
	}
}

// TestGetFrozenMap 冻结时间与多节点查询
func TestGetFrozenMap(t *testing.T) {
	log.SetLevel(log.ERROR)
	setupBlockListDB(t)

	before := time.Now()
	if err := FreezeProxy("1.1.1.1:443"); err != nil {
		t.Fatal(err)
	}
	FreezeProxy("2.2.2.2:443")

	m := GetFrozenMap()
	if len(m) != 2 {
		t.Fatalf("frozen map len = %d, want 2", len(m))
	}
	at, ok := m["1.1.1.1:443"]
	if !ok {
		t.Fatal("缺 1.1.1.1")
	}
	if at.Before(before.Add(-time.Second)) || at.After(time.Now().Add(time.Second)) {
		t.Errorf("FreezeAt 时间异常: %v", at)
	}
}

// TestClearExpiredFreezes 冻结记录超 freeze-window 天被清理（节点不再出现也强制解封）
func TestClearExpiredFreezes(t *testing.T) {
	log.SetLevel(log.ERROR)
	setupBlockListDB(t)

	// 直接插入：一条 31 天前（应被清理），一条 1 天前（应保留）
	old := time.Now().Add(-31 * 24 * time.Hour)
	fresh := time.Now().Add(-24 * time.Hour)
	if err := DB.Create(&ProxyBlockList{Identifier: "old:443", FreezeAt: old}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ProxyBlockList{Identifier: "fresh:443", FreezeAt: fresh}).Error; err != nil {
		t.Fatal(err)
	}

	ClearOldItems()

	if IsFrozen("old:443") {
		t.Error("31 天前的冻结记录应被清理")
	}
	if !IsFrozen("fresh:443") {
		t.Error("1 天前的冻结记录应保留")
	}
}

// TestFreezeIdempotent 重复冻结幂等（更新 FreezeAt 不报错）
func TestFreezeIdempotent(t *testing.T) {
	log.SetLevel(log.ERROR)
	setupBlockListDB(t)

	if err := FreezeProxy("1.1.1.1:443"); err != nil {
		t.Fatal(err)
	}
	if err := FreezeProxy("1.1.1.1:443"); err != nil {
		t.Fatalf("重复冻结应幂等: %v", err)
	}
	if !IsFrozen("1.1.1.1:443") {
		t.Fatal("应仍冻结")
	}
}
