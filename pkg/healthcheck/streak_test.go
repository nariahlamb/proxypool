package healthcheck

import "testing"

// TestRecordHealthResult 验证 streak 状态机：通过/失败互斥累积
func TestRecordHealthResult(t *testing.T) {
	id := "1.1.1.1:443"
	// 连续 2 次失败
	RecordHealthResult(id, false)
	RecordHealthResult(id, false)
	fail, pass := GetStreak(id)
	if fail != 2 || pass != 0 {
		t.Fatalf("after 2 fails: fail=%d pass=%d, want 2/0", fail, pass)
	}
	// 1 次通过 → 失败清零、通过=1
	RecordHealthResult(id, true)
	fail, pass = GetStreak(id)
	if fail != 0 || pass != 1 {
		t.Fatalf("after pass: fail=%d pass=%d, want 0/1", fail, pass)
	}
	// 再失败 → 通过清零、失败=1
	RecordHealthResult(id, false)
	fail, pass = GetStreak(id)
	if fail != 1 || pass != 0 {
		t.Fatalf("after fail again: fail=%d pass=%d, want 1/0", fail, pass)
	}
}

// TestStreakLifecycle 验证不存在的节点返回 0,0 以及删除后归零
func TestStreakLifecycle(t *testing.T) {
	id := "2.2.2.2:443"
	fail, pass := GetStreak(id)
	if fail != 0 || pass != 0 {
		t.Fatalf("unknown node: fail=%d pass=%d, want 0/0", fail, pass)
	}
	RecordHealthResult(id, true)
	RecordHealthResult(id, true)
	if _, p := GetStreak(id); p != 2 {
		t.Fatalf("pass = %d, want 2", p)
	}
	DeleteStreak(id)
	if f, p := GetStreak(id); f != 0 || p != 0 {
		t.Fatalf("after delete: fail=%d pass=%d, want 0/0", f, p)
	}
}

// TestGetStreakSnapshot 快照包含全部记录
func TestGetStreakSnapshot(t *testing.T) {
	RecordHealthResult("3.3.3.3:443", false)
	RecordHealthResult("4.4.4.4:443", true)
	snap := GetStreakSnapshot()
	if _, ok := snap["3.3.3.3:443"]; !ok {
		t.Error("snapshot 缺 3.3.3.3")
	}
	if _, ok := snap["4.4.4.4:443"]; !ok {
		t.Error("snapshot 缺 4.4.4.4")
	}
	DeleteStreak("3.3.3.3:443")
	DeleteStreak("4.4.4.4:443")
}
