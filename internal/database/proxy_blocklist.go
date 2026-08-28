package database

import (
	"time"

	"github.com/One-Piecs/proxypool/log"
	"gorm.io/gorm/clause"
)

// ProxyBlockList 失效节点冻结表：
// 连续健康检查失败达到阈值的节点被冻结，冻结期内即使源站再次返回也不入库；
// 冻结中连续通过达到阈值或超过冻结窗口后强制解锁。
type ProxyBlockList struct {
	Identifier string    `gorm:"primaryKey"`
	FreezeAt   time.Time // 冻结时间（用于 freeze-window 强制解锁）
}

// FreezeProxy 冻结节点（identifier 唯一，重复冻结自动更新 FreezeAt）。
func FreezeProxy(identifier string) error {
	if DB == nil {
		return nil
	}
	rec := ProxyBlockList{Identifier: identifier, FreezeAt: time.Now()}
	if err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "identifier"}},
		DoUpdates: clause.AssignmentColumns([]string{"freeze_at"}),
	}).Create(&rec).Error; err != nil {
		log.Warnln("database: freeze proxy failed: %s", err.Error())
		return err
	}
	return nil
}

// UnfreezeProxy 解冻节点。
func UnfreezeProxy(identifier string) error {
	if DB == nil {
		return nil
	}
	if err := DB.Where("identifier = ?", identifier).Delete(&ProxyBlockList{}).Error; err != nil {
		log.Warnln("database: unfreeze proxy failed: %s", err.Error())
		return err
	}
	return nil
}

// GetFrozenMap 返回全部冻结节点：identifier → 冻结时间。
func GetFrozenMap() map[string]time.Time {
	out := make(map[string]time.Time)
	if DB == nil {
		return out
	}
	var recs []ProxyBlockList
	if err := DB.Find(&recs).Error; err != nil {
		log.Warnln("database: load frozen proxies failed: %s", err.Error())
		return out
	}
	for _, r := range recs {
		out[r.Identifier] = r.FreezeAt
	}
	return out
}

// IsFrozen 判断节点是否处于冻结状态。
func IsFrozen(identifier string) bool {
	if DB == nil {
		return false
	}
	var count int64
	DB.Model(&ProxyBlockList{}).Where("identifier = ?", identifier).Count(&count)
	return count > 0
}
