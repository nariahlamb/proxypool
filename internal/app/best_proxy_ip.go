package app

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/One-Piecs/proxypool/config"
	"github.com/One-Piecs/proxypool/internal/cache"
	"github.com/One-Piecs/proxypool/log"
	"github.com/One-Piecs/proxypool/pkg/cdn"
	"github.com/One-Piecs/proxypool/pkg/geoIp"
	"github.com/One-Piecs/proxypool/pkg/tool"
	"github.com/gammazero/workerpool"
)

type Format struct {
	Surge  bool
	Clash  bool
	QuanX  bool
	Loon   bool
	Vmess  bool
	Trojan bool
	Vless  bool
	Anytls bool
	V2rayn bool
}

func CrawlBestNode() {
	urls := config.Config().SubIpUrl
	addrMap := sync.Map{}
	bestNodeList := make([]cache.BestNode, 0, 200)

	// Fetch from all sources concurrently
	wp := workerpool.New(10)
	wg := &sync.WaitGroup{}

	// 1. Subscription URLs
	if len(urls) > 0 {
		for _, _url := range urls {
			wg.Add(1)
			_url := _url
			wp.Submit(func() {
				defer wg.Done()
				log.Infoln("Starting Sub URL: %s", _url)

				for retries := range 3 {
					resp, err := bestNodeClient.R().
						SetQueryParams(map[string]string{
							"host":       "p.laibbb.top",
							"uuid":       "e4e08238-e42c-4288-8f67-e2994ec18c90",
							"pw":         "e4e08238",
							"path":       "/webhook",
							"edgetunnel": "cmliu",
						}).
						Get(_url)
					if err != nil {
						log.Errorln("resty.Get(): %s, retry: %d", err.Error(), retries)
						// 指数退避：超时后等服务恢复，避免立即重试再超时
						time.Sleep(time.Duration(retries+1) * 2 * time.Second)
						continue
					}

					de64, err := base64.StdEncoding.DecodeString(resp.String())
					if err != nil {
						log.Errorln("url[%s] base64 decode error: %s", _url, err.Error())
						time.Sleep(time.Duration(retries+1) * 2 * time.Second)
						continue
					}

					r := bufio.NewScanner(bytes.NewReader(de64))
					count := 0
					for r.Scan() {
						addr, err := ExtractHostPort(r.Text())
						if err != nil {
							continue
						}
						addrMap.Store(addr, struct{}{})
						count++
					}
					log.Infoln("Processed %s, found %d nodes", _url, count)
					break
				}
			})
		}
	} else {
		log.Errorln("not found sub url")
	}

	// 2. CF Best IPs from Config
	wg.Add(1)
	wp.Submit(func() {
		defer wg.Done()
		cfIps := config.Config().CfBestIp
		if len(cfIps) > 0 {
			log.Infoln("Adding %d CF Best IPs from config", len(cfIps))
			for _, ip := range cfIps {
				// Assuming standard port 443 for these if not specified?
				// SubNiceCfProxyIp passes 443. Let's append default port if missing.
				// However ExtractHostPort might expect port.
				// Let's manually store with :443 if standard IP
				addrMap.Store(normalizeAddr(ip), struct{}{})
			}
		}
	})

	// 3. CF Top 20 from vps789
	wg.Add(1)
	wp.Submit(func() {
		defer wg.Done()
		log.Infoln("Fetching CF Top 20...")
		ips, err := fetchCfIpTop20()
		if err != nil {
			log.Errorln("fetchCfIpTop20 failed: %v", err)
			return
		}
		log.Infoln("Got %d IPs from Top 20", len(ips))
		for _, ip := range ips {
			addrMap.Store(normalizeAddr(ip), struct{}{})
		}
	})

	// 4. CF Provider IPs from vps789
	wg.Add(1)
	wp.Submit(func() {
		defer wg.Done()
		log.Infoln("Fetching CF Provider IPs...")
		// Fetch for all ISPs
		isps := []string{"CT", "CU", "CM"}
		for _, isp := range isps {
			ips, err := fetchCfIpProvider(isp)
			if err != nil {
				log.Errorln("fetchCfIpProvider(%s) failed: %v", isp, err)
				continue
			}
			log.Infoln("Got %d IPs for %s", len(ips), isp)
			for _, ip := range ips {
				addrMap.Store(normalizeAddr(ip), struct{}{})
			}
		}
	})

	// 5. Best IP Sub URLs (明文 ip:port 订阅，如 best-cf-ips)
	subIpListUrls := config.Config().SubIpListUrl
	if len(subIpListUrls) > 0 {
		wg.Add(1)
		wp.Submit(func() {
			defer wg.Done()
			for _, _url := range subIpListUrls {
				log.Infoln("Starting Sub IP Sub URL: %s", _url)

				for retries := range 3 {
					resp, err := bestNodeClient.R().Get(_url)
					if err != nil {
						log.Errorln("bestNodeClient.Get(): %s, retry: %d", err.Error(), retries)
						// 指数退避：超时后等服务恢复，避免立即重试再超时
						time.Sleep(time.Duration(retries+1) * 2 * time.Second)
						continue
					}

					r := bufio.NewScanner(bytes.NewReader([]byte(resp.String())))
					count := 0
					for r.Scan() {
						addr, ok := parseSubIpSubLine(r.Text())
						if !ok {
							continue
						}
						addrMap.Store(addr, struct{}{})
						count++
					}
					log.Infoln("Processed %s, found %d nodes", _url, count)
					break
				}
			}
		})
	} else {
		log.Errorln("not found sub ip sub url")
	}

	wg.Wait()
	wp.Stop()

	// 收集去重后的地址
	addrAll := make([]string, 0, 200)
	addrMap.Range(func(key, value any) bool {
		addrAll = append(addrAll, key.(string))
		return true
	})

	log.Infoln("Total unique addresses: %d", len(addrAll))

	// Resolve domains to IPs
	resolvedAddrAll := make([]string, 0, len(addrAll))
	for _, addr := range addrAll {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			log.Errorln("Failed to split host port for %s: %v", addr, err)
			continue
		}

		if net.ParseIP(host) != nil {
			resolvedAddrAll = append(resolvedAddrAll, addr)
			continue
		}

		// It's a domain, resolve it
		ips, err := net.LookupIP(host)
		if err != nil {
			log.Errorln("Failed to resolve domain %s: %v", host, err)
			continue
		}

		for _, ip := range ips {
			resolvedAddrAll = append(resolvedAddrAll, net.JoinHostPort(ip.String(), port))
		}
	}
	addrAll = resolvedAddrAll
	log.Infoln("Total addresses after resolution: %d", len(addrAll))

	// Pre-process IPs for CDN check
	ipsToCheck := make([]string, 0)
	// Actually we just need a map of IP -> isCDN
	cdnMap := make(map[string]bool)

	for _, addr := range addrAll {
		// Extract IP logic duplicated from below, consider helper or just doing simple parse here
		// For simplicity, let's just do a quick parse or rely on the below loop.
		// But we want to do batch check BEFORE worker pool starts.

		ip, _, err := splitHostPort(addr)
		if err == nil && ip != "" {
			// Priority 1: Check IP Ranges (Fastest, Local)
			if cdn.GlobalManager.IsCDN(ip) {
				cdnMap[ip] = true
				continue
			}

			// Priority 2: Check Local ASN DB (Fast, Local)
			if geoIp.IsCDN(ip) {
				cdnMap[ip] = true
				continue
			}

			// Priority 3: Online API (Slow, External) -> Add to batch list
			ipsToCheck = append(ipsToCheck, ip)
		}
	}

	if len(ipsToCheck) > 0 {
		log.Infoln("Checking ASN for %d IPs", len(ipsToCheck))
		asnResults, err := cdn.CheckIPsForCDN(ipsToCheck)
		if err != nil {
			log.Errorln("ASN check failed: %v", err)
		} else {
			for ip, isCDN := range asnResults {
				if isCDN {
					cdnMap[ip] = true
				}
			}
		}
	}

	// 使用workerpool处理IP检测
	wp = workerpool.New(20)
	mux := sync.Mutex{}

	for _, addr := range addrAll {
		addr := addr // 创建副本
		wp.Submit(func() {
			ip, port, err := splitHostPort(addr)
			if err != nil {
				log.Errorln("invalid addr: %s: %v", addr, err)
				return
			}

			// if ip == "cf.090227.xyz" {
			// 	return
			// }

			_, country, err := geoIp.GeoIpDB.Find(ip)
			if err != nil {
				log.Errorln("GeoIP lookup failed for %s: %s", ip, err.Error())
				return
			}

			// Check if IP is CDN (using pre-calculated map)
			isCDN := cdnMap[ip]

			// 创建节点
			node := cache.BestNode{
				Ip:      ip,
				Port:    port,
				Country: country,
				CDN:     isCDN,
			}

			mux.Lock()
			bestNodeList = append(bestNodeList, node)
			mux.Unlock()

			log.Infoln("Node %s:%d added from %s", ip, port, country)
		})
	}

	wp.StopWait()

	// 按照国家名称、IP和端口多级排序
	slices.SortStableFunc(bestNodeList, func(a, b cache.BestNode) int {
		// 首先按国家排序
		if a.Country != b.Country {
			return cmp.Compare(a.Country, b.Country)
		}
		// 国家相同时按IP排序，使用TCP数值比较
		if a.Ip != b.Ip {
			// 如果是IPv4地址，使用数值比较
			aParts := strings.Split(a.Ip, ".")
			bParts := strings.Split(b.Ip, ".")
			if len(aParts) == 4 && len(bParts) == 4 {
				return cmp.Compare(ipToUint32(a.Ip), ipToUint32(b.Ip))
			}
			// 对于IPv6或其他格式，保持字符串比较
			return cmp.Compare(a.Ip, b.Ip)
		}
		// IP相同时按端口排序
		return cmp.Compare(a.Port, b.Port)
	})

	cache.SetBestNodeList("bestNode", bestNodeList)

	// anytls 可转发性探测：异步执行，完成后原子替换缓存（不阻塞爬取主流程）。
	// 探测用任一配置了 anytls 凭据的国家（默认自动选择），标记的是 ip:port 透传能力。
	if cfg := config.Config().SniProbe; cfg != nil && cfg.Enabled() {
		go func(base []cache.BestNode) {
			marked := ProbeAndMarkAnyTLS(base)
			cache.SetBestNodeList("bestNode", marked)
		}(bestNodeList)
	}
	cache.SetString("bestNodeLastUpdateTime", time.Now().Format(time.RFC3339))
	log.Infoln("Completed processing %d nodes", len(bestNodeList))
}

func SubNiceProxyIp(format string, distNodeCountry string, proxyCountryIsoCode string, limit int, random bool, isIPV6 bool, cdnFilter string) (s string, err error) {
	defer trackDuration("SubNiceProxyIp")()

	// 检查格式并获取配置
	f, err := checkFormat(format, distNodeCountry)
	if err != nil {
		log.Errorln("Format check failed: %v", err)
		return "", fmt.Errorf("format check error: %w", err)
	}

	// 获取并验证节点列表
	bestNodeList := cache.GetBestNodeList("bestNode")
	if len(bestNodeList) == 0 {
		log.Errorln("No best nodes found")
		return "", errors.New("not found best node list")
	}

	// anytls 格式：仅导出可透传 anytls 的节点；未启用探测时导出空
	if f.Anytls {
		bestNodeList = filterAnyTLSNodes(bestNodeList)
	}

	// 预分配buffer以提高性能
	buf := strings.Builder{}
	buf.Grow(len(bestNodeList) * 200) // 预估每个节点约200字节

	writeOutputHeader(&buf, f, cache.GetString("bestNodeLastUpdateTime"))
	// 优化国家代码过滤
	var countryFilter map[string]struct{}
	if proxyCountryIsoCode != "" {
		countryFilter = make(map[string]struct{})
		for _, code := range strings.Split(proxyCountryIsoCode, ",") {
			countryFilter[code] = struct{}{}
		}
	}

	// 复制代理信息以避免并发问题
	proxyInfo, err := loadProxyInfo()
	if err != nil {
		return "", err
	}
	generator := urlGeneratorMap[generatorKey(f)]

	// 按国家分组节点
	countryNodes := make(map[string][]cache.BestNode)
	for _, node := range bestNodeList {
		// 优化的国家过滤逻辑
		if countryFilter != nil {
			matched := false
			for code := range countryFilter {
				if strings.Contains(node.Country, code) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		countryNodes[node.Country] = append(countryNodes[node.Country], node)
	}

	// 处理每个国家的节点，并应用limit限制
	for _, nodes := range countryNodes {
		// 如果random为true，随机打乱节点顺序
		if random {
			rand.Shuffle(len(nodes), func(i, j int) {
				nodes[i], nodes[j] = nodes[j], nodes[i]
			})
		}

		nodeLimit := len(nodes)
		// 仅当limit大于0时才限制节点数量
		if limit > 0 && limit < nodeLimit {
			nodeLimit = limit
		}

		for i := 0; i < nodeLimit; i++ {
			node := nodes[i]

			// Filter based on cdnFilter
			if cdnFilter == "true" && !node.CDN {
				continue
			}
			if cdnFilter == "false" && node.CDN {
				continue
			}

			if generator != nil {
				if isIPV6 && !IsIPv6(node.Ip) {
					continue
				}
				generator(&buf, proxyInfo, distNodeCountry, node.Country, node.Ip, node.Port)
			}
		}
	}

	if f.V2rayn {
		return base64.StdEncoding.EncodeToString([]byte(buf.String())), nil
	}
	return buf.String(), nil
}

func SubNiceCfProxyIp(format string, distNodeCountry string, isIPV6 bool) (s string, err error) {
	defer trackDuration("SubNiceCfProxyIp")()
	f, err := checkFormat(format, distNodeCountry)
	if err != nil {
		log.Errorln("Format check failed: %v", err)
		return "", fmt.Errorf("format check error: %w", err)
	}
	bestCfNodeList := config.Config().CfBestIp
	if len(bestCfNodeList) == 0 {
		log.Errorln("No best cf nodes found")
		return "", errors.New("not found best cf node list")
	}
	proxyInfo, err := loadProxyInfo()
	if err != nil {
		return "", err
	}
	return buildNodeOutput(bestCfNodeList, f, proxyInfo, distNodeCountry, isIPV6, 443), nil
}

// vps789 openapi
type CfIpTop20 struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Count   int    `json:"count"`
	Data    struct {
		Good []struct {
			Id              int     `json:"id"`
			VpsId           int     `json:"vpsId"`
			Ip              string  `json:"ip"`
			AvgLatency      int     `json:"avgLatency"`
			AvgPkgLostRate  float64 `json:"avgPkgLostRate"`
			YdLatency       int     `json:"ydLatency"`
			YdPkgLostRate   int     `json:"ydPkgLostRate"`
			LtLatency       int     `json:"ltLatency"`
			LtPkgLostRate   int     `json:"ltPkgLostRate"`
			DxLatency       int     `json:"dxLatency"`
			DxPkgLostRate   int     `json:"dxPkgLostRate"`
			Label           string  `json:"label"`
			CreatedTime     string  `json:"createdTime"`
			AvgScore        int     `json:"avgScore"`
			YdScore         int     `json:"ydScore"`
			DxScore         int     `json:"dxScore"`
			LtScore         int     `json:"ltScore"`
			HostProvider    string  `json:"hostProvider,omitempty"`
			LocationCountry string  `json:"locationCountry,omitempty"`
			LocationCity    string  `json:"locationCity,omitempty"`
		} `json:"good"`
	} `json:"data"`
}

// SubNiceCfProxyIpTop20 获取 https://vps789.com/openApi/cfIpTop20
func SubNiceCfProxyIpTop20(format string, distNodeCountry string, isConvertIp bool, isIPV6 bool) (s string, err error) {
	defer trackDuration("SubNiceCfProxyIpTop20")()
	f, err := checkFormat(format, distNodeCountry)
	if err != nil {
		log.Errorln("Format check failed: %v", err)
		return "", fmt.Errorf("format check error: %w", err)
	}

	ips, err := fetchCfIpTop20()
	if err != nil {
		log.Errorln("fetchCfIpTop20 failed: %v", err)
		return "", err
	}

	bestCfNodeList := make([]string, 0, 20)
	for _, ip := range ips {
		if isConvertIp {
			resolvedIPs, err := net.LookupIP(ip)
			if err != nil {
				return "", fmt.Errorf("DNS查询失败: %w", err)
			}
			for _, rip := range resolvedIPs {
				bestCfNodeList = append(bestCfNodeList, rip.String())
			}
		} else {
			bestCfNodeList = append(bestCfNodeList, ip)
		}
	}
	bestCfNodeList = Unique(bestCfNodeList)

	proxyInfo, err := loadProxyInfo()
	if err != nil {
		return "", err
	}
	return buildNodeOutput(bestCfNodeList, f, proxyInfo, distNodeCountry, isIPV6, 443), nil
}

type cfIpItem struct {
	Ip               string  `json:"ip"`
	YdLatencyAvg     float64 `json:"ydLatencyAvg"`
	YdPkgLostRateAvg float64 `json:"ydPkgLostRateAvg"`
	LtLatencyAvg     float64 `json:"ltLatencyAvg"`
	LtPkgLostRateAvg float64 `json:"ltPkgLostRateAvg"`
	DxLatencyAvg     float64 `json:"dxLatencyAvg"`
	DxPkgLostRateAvg float64 `json:"dxPkgLostRateAvg"`
	DownloadSpeed    int     `json:"downloadSpeed"`
	CreatedTime      string  `json:"createdTime"`
	AvgScore         int     `json:"avgScore"`
	YdScore          int     `json:"ydScore"`
	DxScore          int     `json:"dxScore"`
	LtScore          int     `json:"ltScore"`
}

type CfIpProvider struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Count   int    `json:"count"`
	Data    struct {
		CT     []cfIpItem `json:"CT"`
		CU     []cfIpItem `json:"CU"`
		CM     []cfIpItem `json:"CM"`
		AllAvg []cfIpItem `json:"AllAvg"`
	} `json:"data"`
}

// SubNiceCfProxyIpProvider 获取 https://vps789.com/openApi/cfIpApi
func SubNiceCfProxyIpProvider(format string, isp string, distNodeCountry string, isIPV6 bool) (s string, err error) {
	defer trackDuration("SubNiceCfProxyIpProvider")()
	f, err := checkFormat(format, distNodeCountry)
	if err != nil {
		log.Errorln("Format check failed: %v", err)
		return "", fmt.Errorf("format check error: %w", err)
	}

	bestCfNodeList, err := fetchCfIpProvider(isp)
	if err != nil {
		log.Errorln("fetchCfIpProvider failed: %v", err)
		return "", err
	}

	proxyInfo, err := loadProxyInfo()
	if err != nil {
		return "", err
	}
	return buildNodeOutput(bestCfNodeList, f, proxyInfo, distNodeCountry, isIPV6, 443), nil
}

// ---------------- Helper Functions for Fetching Data ----------------

func fetchCfIpTop20() ([]string, error) {
	resp, err := tool.GetHttpClient().Get("https://vps789.com/openApi/cfIpTop20")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	var top20 CfIpTop20
	if err := json.Unmarshal(body, &top20); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	var ips []string
	for _, good := range top20.Data.Good {
		ips = append(ips, good.Ip)
	}
	return ips, nil
}

func fetchCfIpProvider(isp string) ([]string, error) {
	resp, err := tool.GetHttpClient().Get("https://vps789.com/openApi/cfIpApi")
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	var provider CfIpProvider
	if err := json.Unmarshal(body, &provider); err != nil {
		return nil, fmt.Errorf("json unmarshal failed: %w", err)
	}

	var targets []cfIpItem

	switch isp {
	case "CT":
		targets = provider.Data.CT
	case "CU":
		targets = provider.Data.CU
	case "CM":
		targets = provider.Data.CM
	default:
		// Collect all
		targets = append(targets, provider.Data.CT...)
		targets = append(targets, provider.Data.CU...)
		targets = append(targets, provider.Data.CM...)
	}

	ips := make([]string, 0, len(targets))
	for _, item := range targets {
		ips = append(ips, item.Ip)
	}
	return ips, nil
}

type nodeBase struct {
	hostname string
	port     string
	fragment string
}

// SubNiceCfProxySub 从 cf sub 订阅连接替换为自己的 IP
func SubNiceCfProxySub(format string, sub string, distNodeCountry string, isIPV6 bool) (s string, err error) {
	defer trackDuration("SubNiceCfProxySub")()

	// 检查格式并获取配置
	f, err := checkFormat(format, distNodeCountry)
	if err != nil {
		log.Errorln("Format check failed: %v", err)
		return "", fmt.Errorf("format check error: %w", err)
	}

	resp, err := tool.GetHttpClient().Get(sub)
	if err != nil {
		log.Errorln("get cf sub failed: %v", err)
		return "", fmt.Errorf("get cf sub failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorln("get cfIpApi readall: %v", err)
		return "", fmt.Errorf("get cfIpApi readall: %w", err)
	}

	addrMap := sync.Map{}

	// 兼容两种订阅格式：
	// 1. base64 编码的 URL 订阅（vless:// 等，原格式）
	// 2. 明文 "host:port#注释" 列表（如 https://bestcf.pages.dev/domain/Domain-Asia.txt）
	if de64, err := base64.StdEncoding.DecodeString(string(body)); err == nil {
		r := bufio.NewScanner(bytes.NewReader(de64))
		for r.Scan() {
			parsedURL, err := url.Parse(r.Text())
			if err != nil {
				log.Errorln("url.Parse: %s", err.Error())
				continue
			}
			// 使用sync.Map进行去重
			addrMap.Store(nodeBase{parsedURL.Hostname(), parsedURL.Port(), parsedURL.Fragment}, struct{}{})
		}
	} else {
		// 明文格式：host:port#注释，端口缺失默认 443
		log.Infoln("url[%s] is not base64, parsing as plain host:port lines", sub)
		r := bufio.NewScanner(bytes.NewReader(body))
		for r.Scan() {
			host, port, name, ok := parsePlainSubLine(r.Text())
			if !ok {
				continue
			}
			addrMap.Store(nodeBase{host, port, name}, struct{}{})
		}
	}

	bestCfNodeList := make([]nodeBase, 0, 200)
	addrMap.Range(func(key, value any) bool {
		bestCfNodeList = append(bestCfNodeList, key.(nodeBase))
		return true
	})

	// 预分配buffer以提高性能
	buf := strings.Builder{}
	buf.Grow(len(bestCfNodeList) * 30) // 预估每个节点约30字节

	writeOutputHeader(&buf, f, time.Now().Format(time.RFC3339))

	// 复制代理信息以避免并发问题
	proxyInfo, err := loadProxyInfo()
	if err != nil {
		return "", err
	}
	generator := urlGeneratorMap2[generatorKey(f)]

	// 处理每个国家的节点，并应用limit限制
	for idx, node := range bestCfNodeList {

		country := geoIp.GeoIpDB.FindCountryIsoEmoji(distNodeCountry)

		port := 443
		if node.port != "" {
			port, err = strconv.Atoi(node.port)
			if err != nil {
				log.Errorln("Failed strconv.Atoi : %v", err)
				continue
			}
		}

		node.fragment = node.fragment + fmt.Sprintf(" %d", idx)

		if generator != nil {
			if isIPV6 && !IsIPv6(node.hostname) {
				continue
			}
			generator(&buf, proxyInfo, distNodeCountry, country, node.fragment, node.hostname, port)
		}
	}

	return finishOutput(&buf, f), nil
}

func checkFormat(format string, distNodeCountry string) (f Format, err error) {
	if strings.Contains(format, "surge") {
		f.Surge = true
	} else if strings.Contains(format, "clash") {
		f.Clash = true
	} else if strings.Contains(format, "quanx") {
		f.QuanX = true
	} else if strings.Contains(format, "loon") {
		f.Loon = true
	} else if strings.Contains(format, "v2rayn") {
		f.V2rayn = true
	} else {
		return f, fmt.Errorf("invaild client format")
	}

	if _, ok := config.Config().ProxyInfo[distNodeCountry]; !ok {
		return f, fmt.Errorf("not found %s node", distNodeCountry)
	}

	if strings.Contains(format, "Vmess") {
		if _, ok := config.Config().ProxyInfo[distNodeCountry]["vmess"]; !ok {
			return f, fmt.Errorf("not found vaild vmess node for country [%s]", distNodeCountry)
		}
		f.Vmess = true
	} else if strings.Contains(format, "Trojan") {
		if _, ok := config.Config().ProxyInfo[distNodeCountry]["trojan"]; !ok {
			return f, fmt.Errorf("not found vaild trojan node for country [%s]", distNodeCountry)
		}
		f.Trojan = true
	} else if strings.Contains(format, "Vless") {
		if _, ok := config.Config().ProxyInfo[distNodeCountry]["vless"]; !ok {
			return f, fmt.Errorf("not found vaild vless node for country [%s]", distNodeCountry)
		}
		f.Vless = true
	} else if strings.Contains(format, "Anytls") {
		if _, ok := config.Config().ProxyInfo[distNodeCountry]["anytls"]; !ok {
			return f, fmt.Errorf("not found vaild anytls node for country [%s], add 'anytls: {host, password}' to proxy_info in config.yaml", distNodeCountry)
		}
		f.Anytls = true
	} else {
		return f, fmt.Errorf("invaild node type")
	}
	return f, nil
}

func ExtractHostPort(link string) (addr string, err error) {
	u, err := url.Parse(link)
	if err != nil {
		return "", err
	}

	return u.Host, nil
}

// bestIpPortWhitelist 端口白名单：best ip 明文订阅仅接受这些端口
var bestIpPortWhitelist = map[string]struct{}{
	"443":  {},
	"2053": {},
	"2083": {},
	"2087": {},
	"2096": {},
	"8443": {},
}

// parseSubIpSubLine 解析 sub_ip_list_url 订阅行。
// 行格式: "ip:port#Country" 或纯 IP 行 "ip#"（无端口，默认补 443），
// # 后为国家/地区注释，可省略，如 "104.17.212.191:443#US 🇺🇸" 或 "47.57.245.232#"。
// 仅接受端口白名单内的条目，host 必须是合法 IP。
func parseSubIpSubLine(line string) (addr string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", false
	}
	// 去掉 # 注释（国家码等）
	if before, _, found := strings.Cut(line, "#"); found {
		line = strings.TrimSpace(before)
	}
	if line == "" {
		return "", false
	}

	host := line
	portStr := "443" // 默认端口：纯 IP 行（如 ipdb bestproxy 源）补默认白名单端口
	if h, p, err := net.SplitHostPort(line); err == nil {
		host, portStr = h, p
	}
	if _, inWhitelist := bestIpPortWhitelist[portStr]; !inWhitelist {
		return "", false
	}
	if net.ParseIP(host) == nil {
		return "", false
	}
	return net.JoinHostPort(host, portStr), true
}

// parsePlainSubLine 解析明文 "host:port#注释" 订阅行（如 bestcf.pages.dev 域名列表）。
// host 可为域名或 IP；端口缺失默认 443；# 后注释作为节点名（可省略）。
// 端口必须位于白名单（443/2053/2083/2087/2096/8443）。
func parsePlainSubLine(line string) (host, port, name string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", "", false
	}
	// # 后为注释（作为节点名）
	if before, after, found := strings.Cut(line, "#"); found {
		name = strings.TrimSpace(after)
		line = strings.TrimSpace(before)
	}
	if line == "" {
		return "", "", "", false
	}

	host = line
	port = ""
	if h, p, err := net.SplitHostPort(line); err == nil {
		host, port = h, p
	}
	if port == "" {
		port = "443" // 默认端口：纯域名/IP 行
	}
	if _, inWhitelist := bestIpPortWhitelist[port]; !inWhitelist {
		return "", "", "", false
	}
	// 裸 IPv6 无方括号：SplitHostPort 失败会把整串当 host，这里丢弃
	if strings.Count(host, ":") > 1 && net.ParseIP(host) == nil {
		return "", "", "", false
	}
	if host == "" {
		return "", "", "", false
	}
	return host, port, name, true
}

func ipToUint32(ip string) uint32 {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return 0
	}
	var result uint32
	for i := range 4 {
		val, err := strconv.Atoi(parts[i])
		if err != nil {
			return 0
		}
		result = result<<8 | uint32(val)
	}
	return result
}

func genSurgeVmessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s:%d = vmess, %-15s, %d, username=%v, sni=%v, ws=true, ws-path=%v, ws-headers=Host:"%v", vmess-aead=true, tls=true
`,
		country, ip, port, ip, port,
		proxyInfo[nodeCountry]["vmess"]["uuid"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		proxyInfo[nodeCountry]["vmess"]["path"],
		proxyInfo[nodeCountry]["vmess"]["host"]))
}

func genSurgeTrojanUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s:%d = trojan, %-15s, %d, password=%v, sni=%v, ws=true, ws-path=%v, ws-headers=Host:"%v"
`,
		country, ip, port, ip, port,
		proxyInfo[nodeCountry]["trojan"]["password"],
		proxyInfo[nodeCountry]["trojan"]["host"],
		proxyInfo[nodeCountry]["trojan"]["path"],
		proxyInfo[nodeCountry]["trojan"]["host"]))
}

func genClashVlessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`  - {"name":"%s %s:%d", "type":"vless", "server":"%s", "port":%d, "uuid":"%v", "network":"ws", "tls":true, "udp":true, "servername":"%v", "client-fingerprint":"chrome", "ws-opts":{"path":"%v", "headers":{"Host":"%v"}}}
`,
		country, ip, port, ip, port,
		proxyInfo[nodeCountry]["vless"]["uuid"],
		proxyInfo[nodeCountry]["vless"]["host"],
		proxyInfo[nodeCountry]["vless"]["path"],
		proxyInfo[nodeCountry]["vless"]["host"]))
}

func genClashVmessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`  - {"name":"%s %s:%d", "type":"vmess", "server":"%s", "port":%d, "uuid":"%v", "tls":true, "cipher":"none", "alterId":0, "network":"ws", "ws-opts":{"path":"%v", "headers":{"Host":"%v"}}, "servername":"%v"}
`,
		country, ip, port, ip, port,
		proxyInfo[nodeCountry]["vmess"]["uuid"],
		proxyInfo[nodeCountry]["vmess"]["path"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		proxyInfo[nodeCountry]["vmess"]["host"]))
}

func genClashTrojanUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, node_country, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`  - {"name":"%s %s:%d", "type":"trojan", "server":"%s", "port":%d, "password":"%v", "sni":"%v", "network":"ws", "ws-opts":{"path":"%v", "headers":{"Host":"%v"}}}
`,
		country, ip, port, ip, port,
		proxyInfo[node_country]["trojan"]["password"],
		proxyInfo[node_country]["trojan"]["host"],
		proxyInfo[node_country]["trojan"]["path"],
		proxyInfo[node_country]["trojan"]["host"]))
}

func genQuanXVlessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`vless = %s:%d, method=none, password=%s, obfs=wss, obfs-uri=%s, obfs-host=%s, tls-verification=false, tls-host=%s, fast-open=false, udp-relay=true, tag=%s %s:%d
`,
		ip, port,
		proxyInfo[nodeCountry]["vless"]["uuid"],
		proxyInfo[nodeCountry]["vless"]["path"],
		proxyInfo[nodeCountry]["vless"]["host"],
		proxyInfo[nodeCountry]["vless"]["host"],
		country, ip, port))
}

func genQuanXVmessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`vmess = %s:%d, method=none, password=%s, obfs=wss, obfs-uri=%s, obfs-host=%s, tls-host=%s, aead=true, udp-relay=true, tag=%s %s:%d
`,
		ip, port,
		proxyInfo[nodeCountry]["vmess"]["uuid"],
		proxyInfo[nodeCountry]["vmess"]["path"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		country, ip, port))
}

func genQuanXTrojanUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`trojan = %s:%d, password=%s, obfs=wss, obfs-uri=%s, obfs-host=%s, tls-host=%s, udp-relay=true, tag=%s %s:%d
`,
		ip, port,
		proxyInfo[nodeCountry]["trojan"]["password"],
		proxyInfo[nodeCountry]["trojan"]["path"],
		proxyInfo[nodeCountry]["trojan"]["host"],
		proxyInfo[nodeCountry]["trojan"]["host"],
		country, ip, port))
}

func genLoonVlessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s:%d = vless, %s, %d, "%s", transport=ws, path=%s, host=%s, udp=true, over-tls=true, sni=%s
`,
		country, ip, port,
		ip, port,
		proxyInfo[nodeCountry]["vless"]["uuid"],
		proxyInfo[nodeCountry]["vless"]["path"],
		proxyInfo[nodeCountry]["vless"]["host"],
		proxyInfo[nodeCountry]["vless"]["host"],
	))
}

func genLoonVmessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s:%d = vmess, %s, %d, none, "%s", transport=ws, alterId=0, path=%s, host=%s, udp=true, over-tls=true, sni=%s
`,
		country, ip, port,
		ip, port,
		proxyInfo[nodeCountry]["vmess"]["uuid"],
		proxyInfo[nodeCountry]["vmess"]["path"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		proxyInfo[nodeCountry]["vmess"]["host"],
	))
}

func genLoonTrojanUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s:%d = trojan, %s, %d, "%s", transport=ws, sni=%s, path=%s, host=%s, udp=true
`,
		country, ip, port,
		ip, port,
		proxyInfo[nodeCountry]["trojan"]["password"],
		proxyInfo[nodeCountry]["trojan"]["host"],
		proxyInfo[nodeCountry]["trojan"]["path"],
		proxyInfo[nodeCountry]["trojan"]["host"],
	))
}

func genSurgeVmessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s = vmess, %-15s, %d, username=%v, sni=%v, ws=true, ws-path=%v, ws-headers=Host:"%v", vmess-aead=true, tls=true
`,
		country, nodeName, ip, port,
		proxyInfo[nodeCountry]["vmess"]["uuid"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		proxyInfo[nodeCountry]["vmess"]["path"],
		proxyInfo[nodeCountry]["vmess"]["host"]))
}

func genSurgeTrojanUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s = trojan, %-15s, %d, password=%v, sni=%v, ws=true, ws-path=%v, ws-headers=Host:"%v"
`,
		country, nodeName, ip, port,
		proxyInfo[nodeCountry]["trojan"]["password"],
		proxyInfo[nodeCountry]["trojan"]["host"],
		proxyInfo[nodeCountry]["trojan"]["path"],
		proxyInfo[nodeCountry]["trojan"]["host"]))
}

func genClashVlessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`  - {"name":"%s %s", "type":"vless", "server":"%s", "port":%d, "uuid":"%v", "network":"ws", "tls":true, "udp":true, "servername":"%v", "client-fingerprint":"chrome", "ws-opts":{"path":"%v", "headers":{"Host":"%v"}}}
`,
		country, nodeName, ip, port,
		proxyInfo[nodeCountry]["vless"]["uuid"],
		proxyInfo[nodeCountry]["vless"]["host"],
		proxyInfo[nodeCountry]["vless"]["path"],
		proxyInfo[nodeCountry]["vless"]["host"]))
}

func genClashVmessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`  - {"name":"%s %s", "type":"vmess", "server":"%s", "port":%d, "uuid":"%v", "tls":true, "cipher":"none", "alterId":0, "network":"ws", "ws-opts":{"path":"%v", "headers":{"Host":"%v"}}, "servername":"%v"}
`,
		country, nodeName, ip, port,
		proxyInfo[nodeCountry]["vmess"]["uuid"],
		proxyInfo[nodeCountry]["vmess"]["path"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		proxyInfo[nodeCountry]["vmess"]["host"]))
}

func genClashTrojanUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, node_country, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`  - {"name":"%s %s", "type":"trojan", "server":"%s", "port":%d, "password":"%v", "sni":"%v", "network":"ws", "ws-opts":{"path":"%v", "headers":{"Host":"%v"}}}
`,
		country, nodeName, ip, port,
		proxyInfo[node_country]["trojan"]["password"],
		proxyInfo[node_country]["trojan"]["host"],
		proxyInfo[node_country]["trojan"]["path"],
		proxyInfo[node_country]["trojan"]["host"]))
}

func genQuanXVlessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`vless = %s:%d, method=none, password=%s, obfs=wss, obfs-uri=%s, obfs-host=%s, tls-verification=false, tls-host=%s, fast-open=false, udp-relay=true, tag=%s %s
`,
		ip, port,
		proxyInfo[nodeCountry]["vless"]["uuid"],
		proxyInfo[nodeCountry]["vless"]["path"],
		proxyInfo[nodeCountry]["vless"]["host"],
		proxyInfo[nodeCountry]["vless"]["host"],
		country, nodeName))
}

func genQuanXVmessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`vmess = %s:%d, method=none, password=%s, obfs=wss, obfs-uri=%s, obfs-host=%s, tls-host=%s, aead=true, udp-relay=true, tag=%s %s
`,
		ip, port,
		proxyInfo[nodeCountry]["vmess"]["uuid"],
		proxyInfo[nodeCountry]["vmess"]["path"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		country, nodeName))
}

func genQuanXTrojanUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`trojan = %s:%d, password=%s, obfs=wss, obfs-uri=%s, obfs-host=%s, tls-host=%s, udp-relay=true, tag=%s %s
`,
		ip, port,
		proxyInfo[nodeCountry]["trojan"]["password"],
		proxyInfo[nodeCountry]["trojan"]["path"],
		proxyInfo[nodeCountry]["trojan"]["host"],
		proxyInfo[nodeCountry]["trojan"]["host"],
		country, nodeName))
}

func genLoonVlessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s = vless, %s, %d, "%s", transport=ws, path=%s, host=%s, udp=true, over-tls=true, sni=%s
`,
		country, nodeName,
		ip, port,
		proxyInfo[nodeCountry]["vless"]["uuid"],
		proxyInfo[nodeCountry]["vless"]["path"],
		proxyInfo[nodeCountry]["vless"]["host"],
		proxyInfo[nodeCountry]["vless"]["host"],
	))
}

func genLoonVmessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s = vmess, %s, %d, none, "%s", transport=ws, alterId=0, path=%s, host=%s, udp=true, over-tls=true, sni=%s
`,
		country, nodeName,
		ip, port,
		proxyInfo[nodeCountry]["vmess"]["uuid"],
		proxyInfo[nodeCountry]["vmess"]["path"],
		proxyInfo[nodeCountry]["vmess"]["host"],
		proxyInfo[nodeCountry]["vmess"]["host"],
	))
}

func genLoonTrojanUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s = trojan, %s, %d, "%s", transport=ws, sni=%s, path=%s, host=%s, udp=true
`,
		country, nodeName,
		ip, port,
		proxyInfo[nodeCountry]["trojan"]["password"],
		proxyInfo[nodeCountry]["trojan"]["host"],
		proxyInfo[nodeCountry]["trojan"]["path"],
		proxyInfo[nodeCountry]["trojan"]["host"],
	))
}

func IsIPv6(addr string) bool {
	ipv6Addr := net.ParseIP(addr)

	// 核心实现：
	//
	// 检查 IP 地址是否为 16 字节长，并且不能被 To4() 成功转换为 IPv4 地址。
	// 如果 To4() 返回非 nil，则表示它是 IPv4 或 IPv4-mapped IPv6 地址。
	// 只有当长度为 16 字节且 To4() 返回 nil 时，才是纯粹的 IPv6 地址。
	return len(ipv6Addr) == net.IPv6len && ipv6Addr.To4() == nil
}

func Unique[T comparable](s []T) []T {
	inGeneric := make(map[T]struct{})
	var result []T
	for _, v := range s {
		if _, ok := inGeneric[v]; !ok {
			inGeneric[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

func genV2raynVmessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	v := make(map[string]any)
	v["v"] = "2"
	v["ps"] = fmt.Sprintf("%s %s:%d", country, ip, port)
	v["add"] = ip
	v["port"] = port
	v["id"] = proxyInfo[nodeCountry]["vmess"]["uuid"]
	v["aid"] = "0"
	v["net"] = "ws"
	v["type"] = "none"
	v["host"] = proxyInfo[nodeCountry]["vmess"]["host"]
	v["path"] = proxyInfo[nodeCountry]["vmess"]["path"]
	v["tls"] = "tls"
	v["sni"] = proxyInfo[nodeCountry]["vmess"]["host"]
	v["fp"] = "chrome"

	jsonStr, _ := json.Marshal(v)
	buf.WriteString("vmess://" + base64.StdEncoding.EncodeToString(jsonStr) + "\n")
}

func genV2raynVmessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	v := make(map[string]any)
	v["v"] = "2"
	v["ps"] = fmt.Sprintf("%s %s", country, nodeName)
	v["add"] = ip
	v["port"] = port
	v["id"] = proxyInfo[nodeCountry]["vmess"]["uuid"]
	v["aid"] = "0"
	v["net"] = "ws"
	v["type"] = "none"
	v["host"] = proxyInfo[nodeCountry]["vmess"]["host"]
	v["path"] = proxyInfo[nodeCountry]["vmess"]["path"]
	v["tls"] = "tls"
	v["sni"] = proxyInfo[nodeCountry]["vmess"]["host"]
	v["fp"] = "chrome"

	jsonStr, _ := json.Marshal(v)
	buf.WriteString("vmess://" + base64.StdEncoding.EncodeToString(jsonStr) + "\n")
}

func genV2raynTrojanUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	// trojan://password@host:port?security=tls&type=ws&path=/path&host=host#name
	host := ip
	if IsIPv6(ip) {
		host = fmt.Sprintf("[%s]", ip)
	}
	u := url.URL{
		Scheme:   "trojan",
		User:     url.User(proxyInfo[nodeCountry]["trojan"]["password"].(string)),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Fragment: fmt.Sprintf("%s %s:%d", country, ip, port),
	}
	q := u.Query()
	q.Set("security", "tls")
	q.Set("type", "ws")
	q.Set("path", proxyInfo[nodeCountry]["trojan"]["path"].(string))
	q.Set("host", proxyInfo[nodeCountry]["trojan"]["host"].(string))
	q.Set("sni", proxyInfo[nodeCountry]["trojan"]["host"].(string))
	q.Set("fp", "chrome")
	u.RawQuery = q.Encode()
	buf.WriteString(u.String() + "\n")
}

func genV2raynTrojanUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	host := ip
	if IsIPv6(ip) {
		host = fmt.Sprintf("[%s]", ip)
	}
	u := url.URL{
		Scheme:   "trojan",
		User:     url.User(proxyInfo[nodeCountry]["trojan"]["password"].(string)),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Fragment: fmt.Sprintf("%s %s", country, nodeName),
	}
	q := u.Query()
	q.Set("security", "tls")
	q.Set("type", "ws")
	q.Set("path", proxyInfo[nodeCountry]["trojan"]["path"].(string))
	q.Set("host", proxyInfo[nodeCountry]["trojan"]["host"].(string))
	q.Set("sni", proxyInfo[nodeCountry]["trojan"]["host"].(string))
	q.Set("fp", "chrome")
	u.RawQuery = q.Encode()
	buf.WriteString(u.String() + "\n")
}

func genV2raynVlessUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	// vless://uuid@host:port?encryption=none&security=tls&type=ws&path=/path&host=host&sni=host#name
	host := ip
	if IsIPv6(ip) {
		host = fmt.Sprintf("[%s]", ip)
	}
	u := url.URL{
		Scheme:   "vless",
		User:     url.User(proxyInfo[nodeCountry]["vless"]["uuid"].(string)),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Fragment: fmt.Sprintf("%s %s:%d", country, ip, port),
	}
	q := u.Query()
	q.Set("encryption", "none")
	q.Set("security", "tls")
	q.Set("type", "ws")
	q.Set("path", proxyInfo[nodeCountry]["vless"]["path"].(string))
	q.Set("host", proxyInfo[nodeCountry]["vless"]["host"].(string))
	q.Set("sni", proxyInfo[nodeCountry]["vless"]["host"].(string))
	q.Set("fp", "chrome")
	u.RawQuery = q.Encode()
	buf.WriteString(u.String() + "\n")
}

func genV2raynVlessUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	host := ip
	if IsIPv6(ip) {
		host = fmt.Sprintf("[%s]", ip)
	}
	u := url.URL{
		Scheme:   "vless",
		User:     url.User(proxyInfo[nodeCountry]["vless"]["uuid"].(string)),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Fragment: fmt.Sprintf("%s %s", country, nodeName),
	}
	q := u.Query()
	q.Set("encryption", "none")
	q.Set("security", "tls")
	q.Set("type", "ws")
	q.Set("path", proxyInfo[nodeCountry]["vless"]["path"].(string))
	q.Set("host", proxyInfo[nodeCountry]["vless"]["host"].(string))
	q.Set("sni", proxyInfo[nodeCountry]["vless"]["host"].(string))
	q.Set("fp", "chrome")
	u.RawQuery = q.Encode()
	buf.WriteString(u.String() + "\n")
}

// ============ anytls 生成器（6 参数版：节点名 = country ip:port） ============

// genSurgeAnytlsUrl surge 格式（iOS 5.17.0+ / Mac 6.4.3+ 支持 AnyTLS v2）
func genSurgeAnytlsUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s:%d = anytls, %-15s, %d, password=%v, sni=%v
`,
		country, ip, port, ip, port,
		proxyInfo[nodeCountry]["anytls"]["password"],
		proxyInfo[nodeCountry]["anytls"]["host"]))
}

// genClashAnytlsUrl clash/mihomo 格式（mihomo 原生支持 anytls）
func genClashAnytlsUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	m := map[string]any{
		"name":     fmt.Sprintf("%s %s:%d", country, ip, port),
		"type":     "anytls",
		"server":   ip,
		"port":     port,
		"password": proxyInfo[nodeCountry]["anytls"]["password"],
		"udp":      true,
	}
	if sni := proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "host"); sni != "" {
		m["sni"] = sni
	}
	if alpn := proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "alpn"); alpn != "" {
		m["alpn"] = strings.Split(alpn, ",")
	}
	if proxyInfoBool(proxyInfo[nodeCountry]["anytls"], "skip_cert_verify") {
		m["skip-cert-verify"] = true
	}
	data, _ := json.Marshal(m)
	buf.WriteString("  - " + string(data) + "\n")
}

// genQuanXAnytlsUrl quanx 格式
func genQuanXAnytlsUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`anytls=%s:%d, password=%v, over-tls=true, udp-relay=true, tls-host=%v, tag=%s %s:%d
`,
		ip, port,
		proxyInfo[nodeCountry]["anytls"]["password"],
		proxyInfo[nodeCountry]["anytls"]["host"],
		country, ip, port))
}

// genLoonAnytlsUrl loon 格式（Loon 3.3+ 支持 anytls）
func genLoonAnytlsUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s:%d = anytls, %s, %d, "%v", over-tls=true, tls-name=%v
`,
		country, ip, port, ip, port,
		proxyInfo[nodeCountry]["anytls"]["password"],
		proxyInfo[nodeCountry]["anytls"]["host"]))
}

// genV2raynAnytlsUrl v2rayN 格式（anytls:// 链接）
func genV2raynAnytlsUrl(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, ip string, port int) {
	host := ip
	if IsIPv6(ip) {
		host = fmt.Sprintf("[%s]", ip)
	}
	u := url.URL{
		Scheme:   "anytls",
		User:     url.User(proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "password")),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Fragment: fmt.Sprintf("%s %s:%d", country, ip, port),
	}
	q := u.Query()
	if sni := proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "host"); sni != "" {
		q.Set("sni", sni)
	}
	if alpn := proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "alpn"); alpn != "" {
		q.Set("alpn", alpn)
	}
	if proxyInfoBool(proxyInfo[nodeCountry]["anytls"], "skip_cert_verify") {
		q.Set("allowInsecure", "1")
	}
	u.RawQuery = q.Encode()
	buf.WriteString(u.String() + "\n")
}

// ============ anytls 生成器（7 参数版：节点名 = nodeName，用于 SubNiceCfProxySub） ============

func genSurgeAnytlsUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s = anytls, %-15s, %d, password=%v, sni=%v
`,
		country, nodeName, ip, port,
		proxyInfo[nodeCountry]["anytls"]["password"],
		proxyInfo[nodeCountry]["anytls"]["host"]))
}

func genClashAnytlsUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	m := map[string]any{
		"name":     fmt.Sprintf("%s %s", country, nodeName),
		"type":     "anytls",
		"server":   ip,
		"port":     port,
		"password": proxyInfo[nodeCountry]["anytls"]["password"],
		"udp":      true,
	}
	if sni := proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "host"); sni != "" {
		m["sni"] = sni
	}
	if alpn := proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "alpn"); alpn != "" {
		m["alpn"] = strings.Split(alpn, ",")
	}
	if proxyInfoBool(proxyInfo[nodeCountry]["anytls"], "skip_cert_verify") {
		m["skip-cert-verify"] = true
	}
	data, _ := json.Marshal(m)
	buf.WriteString("  - " + string(data) + "\n")
}

func genQuanXAnytlsUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`anytls=%s:%d, password=%v, over-tls=true, udp-relay=true, tls-host=%v, tag=%s %s
`,
		ip, port,
		proxyInfo[nodeCountry]["anytls"]["password"],
		proxyInfo[nodeCountry]["anytls"]["host"],
		country, nodeName))
}

func genLoonAnytlsUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	buf.WriteString(fmt.Sprintf(`%s %s = anytls, %s, %d, "%v", over-tls=true, tls-name=%v
`,
		country, nodeName, ip, port,
		proxyInfo[nodeCountry]["anytls"]["password"],
		proxyInfo[nodeCountry]["anytls"]["host"]))
}

func genV2raynAnytlsUrl2(buf *strings.Builder, proxyInfo config.ProxyInfo, nodeCountry, country, nodeName string, ip string, port int) {
	host := ip
	if IsIPv6(ip) {
		host = fmt.Sprintf("[%s]", ip)
	}
	u := url.URL{
		Scheme:   "anytls",
		User:     url.User(proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "password")),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Fragment: fmt.Sprintf("%s %s", country, nodeName),
	}
	q := u.Query()
	if sni := proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "host"); sni != "" {
		q.Set("sni", sni)
	}
	if alpn := proxyInfoStr(proxyInfo[nodeCountry]["anytls"], "alpn"); alpn != "" {
		q.Set("alpn", alpn)
	}
	if proxyInfoBool(proxyInfo[nodeCountry]["anytls"], "skip_cert_verify") {
		q.Set("allowInsecure", "1")
	}
	u.RawQuery = q.Encode()
	buf.WriteString(u.String() + "\n")
}
