package database

import (
	"testing"
	"time"

	"github.com/One-Piecs/proxypool/log"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// setupBlockListDB 初始化内存 sqlite 并迁移冻结表
func setupBlockListDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ProxyBlockList{}); err != nil {
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
