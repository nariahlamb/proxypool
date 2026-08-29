package cdn

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/One-Piecs/proxypool/log"
)

// IP-API.com batch endpoint
const ipApiBatchUrl = "http://ip-api.com/batch"

// asnCheckCache 记录 ip-api 查询结果，避免每个周期都重复打外部接口，
// 也能降低外部接口抖动导致的同一 IP 判定不稳定。
var (
	asnCheckCacheMu sync.Mutex
	asnCheckCache   = make(map[string]asnCheckCacheEntry)
	asnCheckCacheTTL = 30 * time.Minute
)

type asnCheckCacheEntry struct {
	isCDN bool
	exp   time.Time
}

type IPAPIResponse struct {
	Query  string `json:"query"`
	Status string `json:"status"`
	ISP    string `json:"isp"`
	Org    string `json:"org"`
	AS     string `json:"as"`
}

// CheckIPsForCDN uses ip-api.com batch API to check if IPs are related to CDNs
// It limits requests to 100 IPs per batch.
func CheckIPsForCDN(ips []string) (map[string]bool, error) {
	results := make(map[string]bool)
	chunkSize := 100

	// 1) 命中缓存（未过期）的直接返回；去重待查询列表
	now := time.Now()
	asnCheckCacheMu.Lock()
	toFetch := make([]string, 0)
	seen := make(map[string]struct{})
	for _, ip := range ips {
		if e, ok := asnCheckCache[ip]; ok && e.exp.After(now) {
			results[ip] = e.isCDN
			continue
		}
		if _, dup := seen[ip]; dup {
			continue
		}
		seen[ip] = struct{}{}
		toFetch = append(toFetch, ip)
	}
	asnCheckCacheMu.Unlock()

	if len(toFetch) == 0 {
		return results, nil
	}

	// 2) 分批查询，并写入缓存（仅成功的批次，避免失败污染缓存）
	for i := 0; i < len(toFetch); i += chunkSize {
		end := i + chunkSize
		if end > len(toFetch) {
			end = len(toFetch)
		}

		batchIPs := toFetch[i:end]
		batchResults, err := fetchIPAPIBatch(batchIPs)
		if err != nil {
			log.Errorln("fetchIPAPIBatch failed: %v", err)
			continue
		}

		asnCheckCacheMu.Lock()
		for ip, isCDN := range batchResults {
			results[ip] = isCDN
			asnCheckCache[ip] = asnCheckCacheEntry{isCDN: isCDN, exp: now.Add(asnCheckCacheTTL)}
		}
		asnCheckCacheMu.Unlock()
	}
	return results, nil
}

func fetchIPAPIBatch(ips []string) (map[string]bool, error) {
	payload, err := json.Marshal(ips)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(ipApiBatchUrl, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResps []IPAPIResponse
	if err := json.Unmarshal(body, &apiResps); err != nil {
		return nil, err
	}

	results := make(map[string]bool)
	for _, info := range apiResps {
		results[info.Query] = isCDNInfo(info)
	}
	return results, nil
}

func isCDNInfo(info IPAPIResponse) bool {
	combined := strings.Join([]string{info.ISP, info.Org, info.AS}, " ")
	combined = strings.ToUpper(combined)
	return MatchOrg(combined)
}
