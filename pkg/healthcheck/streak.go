package healthcheck

import "sync"

// StreakState 单个节点的连续失败/通过轮数。
// 用于失效节点冻结机制：连续失败达到阈值 → 冻结（不入库）；
// 冻结中连续通过达到阈值 → 解锁（允许恢复入库）。
type StreakState struct {
	FailStreak uint16 // 连续健康检查失败轮数
	PassStreak uint16 // 连续健康检查通过轮数
}

var (
	streakMu  sync.RWMutex
	streakMap = make(map[string]*StreakState)
)

// RecordHealthResult 记录一次健康检查结果：
// 通过 → PassStreak+1 且 FailStreak 清零；失败 → FailStreak+1 且 PassStreak 清零。
func RecordHealthResult(id string, ok bool) {
	streakMu.Lock()
	defer streakMu.Unlock()
	s := streakMap[id]
	if s == nil {
		s = &StreakState{}
		streakMap[id] = s
	}
	if ok {
		s.PassStreak++
		s.FailStreak = 0
	} else {
		s.FailStreak++
		s.PassStreak = 0
	}
}

// GetStreak 返回节点的连续失败/通过轮数（不存在返回 0,0）。
func GetStreak(id string) (fail, pass uint16) {
	streakMu.RLock()
	defer streakMu.RUnlock()
	if s := streakMap[id]; s != nil {
		return s.FailStreak, s.PassStreak
	}
	return 0, 0
}

// DeleteStreak 删除节点 streak 记录（节点不再出现且未冻结时调用，防止内存增长）。
func DeleteStreak(id string) {
	streakMu.Lock()
	defer streakMu.Unlock()
	delete(streakMap, id)
}

// GetStreakSnapshot 返回当前全部 streak 的快照（供状态机遍历）。
func GetStreakSnapshot() map[string]StreakState {
	streakMu.RLock()
	defer streakMu.RUnlock()
	out := make(map[string]StreakState, len(streakMap))
	for id, s := range streakMap {
		out[id] = *s
	}
	return out
}
