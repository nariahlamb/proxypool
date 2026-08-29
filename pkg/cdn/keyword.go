package cdn

import "strings"

// cdnGenericUpper 通用且易误伤的词：仅在作为独立词出现时才认定 CDN（如 EDGE、CACHE）。
var cdnGenericUpper = []string{"CDN", "EDGE", "ANYCAST", "CACHE"}

// cdnBrandUpper 特征明显的 CDN/云厂商名（含短语）：子串命中即可。
// 保留云厂商是为了命中其边缘分发（如 Amazon → CloudFront、Google → GCP CDN）。
var cdnBrandUpper = []string{
	"CONTENT DELIVERY", "AKAMAI", "INCAP", "STACKPATH", "BUNNY", "ZSCALER",
	"CLOUDFLARE", "FASTLY", "MICROSOFT", "AZURE", "AMAZON", "GOOGLE", "EDGIO",
	"EDGECAST", "LIMELIGHT", "CACHEFLY", "CDNETWORKS", "ARVANCLOUD", "TENCENT", "ALIBABA",
}

// MatchOrg 判断组织名/ISP/AS 拼接文本（请先转大写）是否属于 CDN：
//   - 通用词用词边界匹配，避免 "CACHE"/"EDGE" 命中 "CACHEXX"/"EDGEXX" 等非 CDN 名称；
//   - 品牌名用子串匹配，保留 CACHEFLY / EDGECAST / 云厂商等真实相关方。
func MatchOrg(upperOrg string) bool {
	for _, kw := range cdnGenericUpper {
		if containsWord(upperOrg, kw) {
			return true
		}
	}
	for _, kw := range cdnBrandUpper {
		if strings.Contains(upperOrg, kw) {
			return true
		}
	}
	return false
}

// containsWord 在 uppercase 文本中按词边界查找 word（ASCII 字母/数字/连字符视为词字符）。
func containsWord(upper, word string) bool {
	start := 0
	for {
		i := strings.Index(upper[start:], word)
		if i < 0 {
			return false
		}
		pos := start + i
		beforeOK := pos == 0 || !isWordChar(upper[pos-1])
		afterOK := pos+len(word) == len(upper) || !isWordChar(upper[pos+len(word)])
		if beforeOK && afterOK {
			return true
		}
		start = pos + len(word)
	}
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-'
}
