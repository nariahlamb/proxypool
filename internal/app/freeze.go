package app

import (
	"time"

	"github.com/One-Piecs/proxypool/config"
	"github.com/One-Piecs/proxypool/internal/database"
	"github.com/One-Piecs/proxypool/log"
	"github.com/One-Piecs/proxypool/pkg/healthcheck"
	"github.com/One-Piecs/proxypool/pkg/proxy"
)

// updateFreezeState 每轮抓取后更新失效节点冻结状态（解决"失效节点被源站持续返回
// 导致永远留库"问题——清理只按"是否被抓取"判断，冻结机制补充"历史健康检查失败"维度）：
//   - 本轮出现且连续失败 >= freeze-failures → 冻结（持久化到 proxy_blocklist）
//   - 冻结中连续通过 >= unlock-passes → 解冻（节点真正恢复才允许回归）
//   - 冻结超过 freeze-window 天 → 强制解冻（防止节点恢复后永久封禁）
//   - 清理不再出现且未冻结节点的 streak 记录（防内存增长）
func updateFreezeState(roundIDs []string) {
	cfg := config.Config()
	freezeFailures := cfg.FreezeFailures
	unlockPasses := cfg.UnlockPasses
	freezeWindow := time.Duration(cfg.FreezeWindow) * 24 * time.Hour

	frozen := database.GetFrozenMap()
	now := time.Now()

	roundSet := make(map[string]struct{}, len(roundIDs))
	for _, id := range roundIDs {
		roundSet[id] = struct{}{}
		fail, pass := healthcheck.GetStreak(id)
		if freezeAt, isFrozen := frozen[id]; isFrozen {
			if pass >= uint16(unlockPasses) || now.Sub(freezeAt) > freezeWindow {
				database.UnfreezeProxy(id)
				log.Infoln("freeze: unfreeze %s (pass=%d, frozen since %s)", id, pass, freezeAt.Format("2006-01-02"))
			}
		} else if fail >= uint16(freezeFailures) {
			database.FreezeProxy(id)
			log.Warnln("freeze: freeze %s (fail streak=%d)", id, fail)
		}
	}

	// 清理不再出现且未冻结的 streak 记录
	for id := range healthcheck.GetStreakSnapshot() {
		if _, inRound := roundSet[id]; !inRound {
			if _, isFrozen := frozen[id]; !isFrozen {
				healthcheck.DeleteStreak(id)
			}
		}
	}
}

// filterFrozenProxies 剔除冻结中的节点（冻结节点不入库、不参与命名/中转/OpenAI 等后续流程；
// 其健康检查 streak 已在 CleanBadProxies 中积累，冻结中连续通过即可解锁恢复）。
func filterFrozenProxies(proxies proxy.ProxyList) proxy.ProxyList {
	frozen := database.GetFrozenMap()
	if len(frozen) == 0 {
		return proxies
	}
	out := make(proxy.ProxyList, 0, len(proxies))
	for _, p := range proxies {
		if _, ok := frozen[p.Identifier()]; !ok {
			out = append(out, p)
		}
	}
	return out
}
