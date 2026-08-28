package database

// UseableStats 数据库中的可用（useable=true）节点统计：
// 启动时展示"上次可用节点"的真实数量，区别于全部节点（含历史失效）。
type UseableStats struct {
	Total   int
	ByType  map[string]int // type -> 数量
}

// CountUseableStats 统计 useable=true 的节点总数与按类型分布。
func CountUseableStats() UseableStats {
	out := UseableStats{ByType: make(map[string]int)}
	if DB == nil {
		return out
	}
	var rows []struct {
		Type string
		Cnt  int
	}
	if err := DB.Model(&Proxy{}).
		Select("type, count(*) as cnt").
		Where("useable = ?", true).
		Group("type").Scan(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		out.ByType[r.Type] = r.Cnt
		out.Total += r.Cnt
	}
	return out
}
