package database

import (
	"time"

	"github.com/One-Piecs/proxypool/internal/cache"
	"github.com/One-Piecs/proxypool/log"
	"gorm.io/gorm"
)

// BestNodeDB best 节点（优选 IP）持久化表。
// 与内存缓存 cache.BestNode 对应，供重启后秒级恢复；
// 存储策略为全量覆盖（每轮采集/探测后整表替换），表大小恒定不增长。
type BestNodeDB struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Ip      string
	Port    int
	Country string
	CDN     bool
	AnyTLS  bool
	Healthy bool
}

// SaveBestNodes 全量保存 best 节点列表（事务：清空 + 批量插入）。
// 探测标记（AnyTLS/Healthy）随列表一起持久化。
func SaveBestNodes(nodes []cache.BestNode) error {
	if DB == nil {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&BestNodeDB{}).Error; err != nil {
			log.Warnln("database: clear best nodes failed: %s", err.Error())
			return err
		}
		if len(nodes) == 0 {
			return nil
		}
		recs := make([]BestNodeDB, len(nodes))
		for i, n := range nodes {
			recs[i] = BestNodeDB{
				Ip:      n.Ip,
				Port:    n.Port,
				Country: n.Country,
				CDN:     n.CDN,
				AnyTLS:  n.AnyTLS,
				Healthy: n.Healthy,
			}
		}
		if err := tx.CreateInBatches(recs, 500).Error; err != nil {
			log.Warnln("database: save best nodes failed: %s", err.Error())
			return err
		}
		log.Infoln("database: saved %d best nodes", len(recs))
		return nil
	})
}

// LoadBestNodes 从数据库加载全部 best 节点（含探测标记）。
func LoadBestNodes() []cache.BestNode {
	if DB == nil {
		return nil
	}
	var recs []BestNodeDB
	if err := DB.Find(&recs).Error; err != nil {
		log.Warnln("database: load best nodes failed: %s", err.Error())
		return nil
	}
	out := make([]cache.BestNode, len(recs))
	for i, r := range recs {
		out[i] = cache.BestNode{
			Ip:      r.Ip,
			Port:    r.Port,
			Country: r.Country,
			CDN:     r.CDN,
			AnyTLS:  r.AnyTLS,
			Healthy: r.Healthy,
		}
	}
	return out
}
