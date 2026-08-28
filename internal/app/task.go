package app

import (
	"fmt"
	"sync"
	"time"

	"github.com/One-Piecs/proxypool/pkg/geoIp"

	"github.com/One-Piecs/proxypool/config"
	"github.com/One-Piecs/proxypool/internal/cache"
	"github.com/One-Piecs/proxypool/internal/database"
	"github.com/One-Piecs/proxypool/log"
	"github.com/One-Piecs/proxypool/pkg/healthcheck"
	"github.com/One-Piecs/proxypool/pkg/provider"
	"github.com/One-Piecs/proxypool/pkg/proxy"
)

var location, _ = time.LoadLocation("PRC")

func CrawlGo() {
	// 健康检查测试地址：config `healthcheck_test_urls` 可覆盖默认列表
	// （默认国内可达 204 端点优先，不含 gstatic）。空列表 = 使用默认。
	if urls := config.Config().HealthcheckTestURLs; len(urls) > 0 {
		healthcheck.SetTestURLs(urls)
	} else {
		healthcheck.SetTestURLs(nil) // 恢复默认（热更新配置后生效）
	}

	wg := &sync.WaitGroup{}
	pc := make(chan proxy.Proxy)
	for _, g := range Getters {
		wg.Go(func() { g.Get2Chan(pc) })
	}
	proxies := cache.GetProxies("allproxies")
	dbProxies := database.GetAllProxies()
	// Show last time result when launch
	if proxies == nil && dbProxies != nil {
		cache.SetProxies("proxies", dbProxies)
		cache.LastCrawlTime = "抓取中，已载入上次数据库数据"
		log.Infoln("Database loaded count: %d", len(dbProxies))
		// 同步首页统计（避免启动窗口期全部显示 0）：以库内节点作为启动快照
		cache.AllProxiesCount = dbProxies.Len()
		cache.UsefullProxiesCount = dbProxies.Len()
		ss, ssr, vmess, trojan, vless, anytls := dbProxies.TypeCounts()
		cache.SSProxiesCount, cache.UsefullSSProxiesCount = ss, ss
		cache.SSRProxiesCount, cache.UsefullSSRProxiesCount = ssr, ssr
		cache.VmessProxiesCount, cache.UsefullVmessProxiesCount = vmess, vmess
		cache.TrojanProxiesCount, cache.UsefullTrojanProxiesCount = trojan, trojan
		cache.VlessProxiesCount, cache.UsefullVlessProxiesCount = vless, vless
		cache.AnyTLSProxiesCount, cache.UsefullAnyTLSProxiesCount = anytls, anytls
	}
	if dbProxies != nil {
		proxies = dbProxies.UniqAppendProxyList2(proxies)
	}
	if proxies == nil {
		proxies = make(proxy.ProxyList, 0)
	}

	go func() {
		wg.Wait()
		close(pc)
	}() // Note: 为何并发？可以一边抓取一边读取而非抓完再读

	// 接收新增 proxy, 去重
	mp := proxies.ToProxyMap()
	for p := range pc { // Note: pc关闭后不能发送数据可以读取剩余数据
		if p != nil {
			mp.UniqAppendProxy(p)
		}
	}
	proxies = mp.ToProxyList()

	// proxies.NameClear()
	proxies = proxies.Derive()
	log.Infoln("CrawlGo unique proxy count: %d", len(proxies))

	// Clean Clash unsupported proxy because health check depends on clash
	proxies = provider.Clash{
		Base: provider.Base{
			Proxies: &proxies,
		},
	}.CleanProxies()
	log.Infoln("CrawlGo clash supported proxy count: %d", len(proxies))

	cache.SetProxies("allproxies", proxies)
	cache.AllProxiesCount = proxies.Len()
	log.Infoln("AllProxiesCount: %d", cache.AllProxiesCount)
	// 单次遍历统计各类型数量
	ssCount, ssrCount, vmessCount, trojanCount, vlessCount, anytlsCount := proxies.TypeCounts()
	cache.SSProxiesCount = ssCount
	log.Infoln("SSProxiesCount: %d", cache.SSProxiesCount)
	cache.SSRProxiesCount = ssrCount
	log.Infoln("SSRProxiesCount: %d", cache.SSRProxiesCount)
	cache.VmessProxiesCount = vmessCount
	log.Infoln("VmessProxiesCount: %d", cache.VmessProxiesCount)
	cache.TrojanProxiesCount = trojanCount
	log.Infoln("TrojanProxiesCount: %d", cache.TrojanProxiesCount)
	cache.VlessProxiesCount = vlessCount
	log.Infoln("VlessProxiesCount: %d", cache.VlessProxiesCount)
	cache.AnyTLSProxiesCount = anytlsCount
	log.Infoln("AnyTLSProxiesCount: %d", cache.AnyTLSProxiesCount)
	cache.LastCrawlTime = time.Now().In(location).Format("2006-01-02 15:04:05")

	// 节点可用性检测，使用batchsize不能降低内存占用，只是为了看性能
	log.Infoln("Now proceed proxy health check...")
	/*
		b := 1000
		round := len(proxies) / b
		okproxies := make(proxy.ProxyList, 0)
		for i := 0; i < round; i++ {
			okproxies = append(okproxies, healthcheck.CleanBadProxiesWithWorkpool(proxies[i*b:(i+1)*b])...)
			log.Infoln("\tChecking round: %d", i)
		}
		okproxies = append(okproxies, healthcheck.CleanBadProxiesWithWorkpool(proxies[round*b:])...)
		proxies = okproxies
	*/
	// 本轮全部节点 id（健康检查前快照，供冻结状态机使用）
	roundIDs := make([]string, 0, len(proxies))
	for _, p := range proxies {
		roundIDs = append(roundIDs, p.Identifier())
	}

	proxies = healthcheck.CleanBadProxiesWithWorkpool(proxies)

	// 失效节点冻结状态机：连续失败达阈值 → 冻结；冻结中连续通过/超窗口 → 解冻
	updateFreezeState(roundIDs)

	// 冻结节点即使本轮通过健康检查也不参与后续流程与入库（streak 仍会积累，用于解锁判定）
	proxies = filterFrozenProxies(proxies)

	log.Infoln("CrawlGo clash usable proxy count: %d", len(proxies))

	// 重命名节点名称为类似US_01的格式，并按国家排序
	proxies.NameClear()
	proxies.NameAddCountry().Sort()
	log.Infoln("Proxy rename DONE!")
	/*
		// 中转检测并命名
		healthcheck.RelayCheck(proxies)
		for i, _ := range proxies {
			if s, ok := healthcheck.ProxyStats.Find(proxies[i]); ok {
				if s.Relay == true {
					_, c, e := geoIp.GeoIpDB.Find(s.OutIp)
					if e == nil {
						proxies[i].SetName(fmt.Sprintf("Relay_%s-%s", proxies[i].BaseInfo().Name, c))
					}
				} else if s.Pool == true {
					proxies[i].SetName(fmt.Sprintf("Pool_%s", proxies[i].BaseInfo().Name))
				}
			}
		}
	*/
	// 中转检测并命名
	healthcheck.RelayCheckWorkpool(proxies)
	for i := range proxies {
		if s, ok := healthcheck.FindStat(proxies[i]); ok {
			if s.Relay {
				_, c, e := geoIp.GeoIpDB.Find(s.OutIp)
				if e == nil {
					// proxies[i].SetName(fmt.Sprintf("Relay_%s-%s", proxies[i].BaseInfo().Name, c))

					if proxies[i].BaseInfo().Name == "🇨🇳 CN" {
						// proxies[i].SetCountry(fmt.Sprintf("%s-%s", proxies[i].BaseInfo().Name, c))
						// proxies[i].SetName(fmt.Sprintf("Relay_%s-%s", proxies[i].BaseInfo().Name, c))

						proxies[i].SetCountry(c)
						proxies[i].SetName(fmt.Sprintf("Relay %s", c))
					} else {
						proxies[i].SetCountry(c)
						proxies[i].SetName(c)
					}
				}
				// }
				// else if s.Pool {
				// proxies[i].SetName(fmt.Sprintf("Pool_%s", proxies[i].BaseInfo().Name))
			}
		}
	}

	// 检测是否支持ChatGPT
	healthcheck.CheckWorkpool(proxies)
	for i := range proxies {
		if s, ok := healthcheck.FindStat(proxies[i]); ok {
			if s.ChatGPT {
				proxies[i].SetName(fmt.Sprintf("ChatGPT %s", proxies[i].BaseInfo().Name))
			}
		}
	}

	proxies.Sort().NameAddIndex()

	// 可用节点存储
	cache.SetProxies("proxies", proxies)
	cache.UsefullProxiesCount = proxies.Len()
	// 单次遍历统计各类型数量（变量已在上面声明）
	ssCount, ssrCount, vmessCount, trojanCount, vlessCount, anytlsCount = proxies.TypeCounts()
	cache.UsefullSSRProxiesCount = ssrCount
	cache.UsefullSSProxiesCount = ssCount
	cache.UsefullVmessProxiesCount = vmessCount
	cache.UsefullTrojanProxiesCount = trojanCount
	cache.UsefullVlessProxiesCount = vlessCount
	cache.UsefullAnyTLSProxiesCount = anytlsCount
	database.SaveProxyList(proxies)
	// database.SaveBlockProxyList(healthcheck.ProxyInvalidStats)
	database.ClearOldItems()

	log.Infoln("Usablility checking done. Open %s to check", config.Config().Domain+":"+config.Config().Port)

	// 测速
	speedTestNew(proxies)
	RefreshProviderCache(proxies)
}

// Speed test for new proxies
func speedTestNew(proxies proxy.ProxyList) {
	if config.Config().SpeedTest {
		cache.IsSpeedTest = "已开启"
		if config.Config().Timeout > 0 {
			healthcheck.SpeedTimeout = time.Second * time.Duration(config.Config().Timeout)
		}
		healthcheck.SpeedTestNewWithWorkpool(proxies, config.Config().Connection)
		// 测速结果持久化到数据库，重启后恢复
		database.SaveProxiesSpeed(proxies)
	} else {
		cache.IsSpeedTest = "未开启"
	}
}

// Speed test for all proxies in proxy.ProxyList
func SpeedTest(proxies proxy.ProxyList) {
	if config.Config().SpeedTest {
		cache.IsSpeedTest = "已开启"
		if config.Config().Timeout > 0 {
			healthcheck.SpeedTimeout = time.Second * time.Duration(config.Config().Timeout)
		}
		healthcheck.SpeedTestAllWithWorkpool(proxies, config.Config().Connection)
		// 测速结果持久化到数据库，重启后恢复
		database.SaveProxiesSpeed(proxies)
	} else {
		cache.IsSpeedTest = "未开启"
	}
}
