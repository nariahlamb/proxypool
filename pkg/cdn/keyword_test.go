package cdn

import "testing"

func TestMatchOrg(t *testing.T) {
	cases := []struct {
		org  string
		want bool
	}{
		// 品牌 / 厂商：子串命中
		{"CLOUDFLARE, INC.", true},
		{"AKAMAI TECHNOLOGIES, INC.", true},
		{"AMAZON.COM, INC.", true},
		{"GOOGLE LLC", true},
		{"FASTLY, INC.", true},
		{"CACHEFLY", true},
		{"EDGECAST NETWORKS", true},
		{"CONTENT DELIVERY NETWORK", true},
		// 通用词按词边界：独立成词才命中
		{"CLOUDFLARE CDN K.K.", true}, // CDN 独立成词
		{"XXCDNXX", false},            // CDN 不是独立词
		{"EDGEHOSTING", false},        // EDGE 不是独立词
		{"CACHEBOX", false},           // CACHE 不是独立词
		// 非 CDN
		{"SOME NICE HOSTING", false},
		{"MY ISP", false},
	}
	for _, c := range cases {
		if got := MatchOrg(c.org); got != c.want {
			t.Errorf("MatchOrg(%q) = %v, want %v", c.org, got, c.want)
		}
	}
}
