package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"sync"
	"strings"
	"time"

	"github.com/One-Piecs/proxypool/internal/app"

	"github.com/arl/statsviz"

	"github.com/One-Piecs/proxypool/log"
	"github.com/One-Piecs/proxypool/pkg/geoIp"
	"github.com/One-Piecs/proxypool/pkg/provider"

	"github.com/One-Piecs/proxypool/config"
	appcache "github.com/One-Piecs/proxypool/internal/cache"
	"github.com/One-Piecs/proxypool/pkg/proxy"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
)

var (
	version string
	router  *gin.Engine
)

// clientProtocols 各客户端实际支持的协议（与 internal/app 生成器矩阵一致：Surge 无 vless 生成器）
var clientProtocols = map[string][]string{
	"clash":  {"vless", "vmess", "trojan", "anytls"},
	"surge":  {"vmess", "trojan", "anytls"},
	"loon":   {"vless", "vmess", "trojan", "anytls"},
	"quanx":  {"vless", "vmess", "trojan", "anytls"},
	"v2rayn": {"vless", "vmess", "trojan", "anytls"},
}

// clientPageRow 客户端页配置/节点列表的通用行
type clientPageRow struct {
	Label      string // 行标题
	Text       string // 静态文本（Text != "" 时优先于 Rel）
	Rel        string // data-sub-path（JS 填充 URL + 复制按钮）
	LinkScheme string // 一键导入深链 scheme（如 clash://install-config），渲染时拼完整地址
	LinkLabel  string // 深链按钮文字
	LinkURL    template.URL // 渲染后的一键导入完整深链（template.URL 绕过 URL 安全过滤，自定义 scheme 如 surge3://）
}

// clientPage 客户端页渲染数据（合并 6 个客户端页为统一 client.html 模板）
type clientPage struct {
	Name       string
	Title      string
	Icon       template.HTML
	Desc       string
	Note       string
	ConfigRows []clientPageRow
	NodeRows   []clientPageRow
}

var clientPages = map[string]clientPage{
	"clash": {
		Name:  "clash",
		Title: "Clash - Free Proxies",
		Icon:  `<i aria-hidden="true" class="metron-clash text-2xl"></i>`,
		ConfigRows: []clientPageRow{
			{Label: "远程配置文件", Rel: "/clash/config", LinkScheme: "clash://install-config", LinkLabel: "一键导入"},
			{Label: "本地部署时配置文件", Text: "LOCAL", LinkScheme: "clash://install-config", LinkLabel: "一键导入"},
		},
		NodeRows: []clientPageRow{{Label: "所有节点", Rel: "/clash/proxies"}},
	},
	"surge": {
		Name:  "surge",
		Title: "Surge - Free Proxies",
		Icon:  `<i aria-hidden="true" class="metron-surge text-2xl"></i>`,
		Desc:  "Surge 配置文件与节点订阅",
		Note:  `Surge 导入方法：点击「一键导入」自动跳转安装配置；或手动在 <b>设置 → 配置 → 从 URL 下载配置</b> 填入配置地址。节点列表也可粘贴到 <b>代理 → 手动添加</b> 使用。支持 type（类型）、c（国家）、speed、filter、tls、reality 等筛选参数。`,
		ConfigRows: []clientPageRow{
			{Label: "Surge 配置文件（含节点与规则）", Rel: "/surge/config", LinkScheme: "surge3:///install-config", LinkLabel: "一键导入"},
		},
		NodeRows: []clientPageRow{
			{Label: "全部节点订阅", Rel: "/surge/proxies"},
			{Label: "按类型订阅", Rel: "/surge/proxies?type=vless"},
			{Label: "常用国家订阅", Rel: "/surge/proxies?c=CN,HK,TW,JP,US"},
		},
	},
	"loon": {
		Name:  "loon",
		Title: "Loon - Free Proxies",
		Icon:  `<span class="inline-flex items-center justify-center w-7 h-7 rounded-lg bg-indigo-100 dark:bg-indigo-900 text-indigo-700 dark:text-indigo-300 text-base font-bold">L</span>`,
		Desc:  "Loon 节点订阅",
		Note:  `Loon 导入方法：打开 Loon，进入 <b>配置 → 订阅 → 远程订阅</b>，粘贴上方订阅地址保存后拉取；或在 <b>节点</b> 页右上角添加订阅。支持 type（类型）、c（国家）、speed、filter、tls、reality 等筛选参数。`,
		NodeRows: []clientPageRow{
			{Label: "全部节点订阅", Rel: "/loon/proxies"},
			{Label: "按类型订阅", Rel: "/loon/proxies?type=vless"},
			{Label: "常用国家订阅", Rel: "/loon/proxies?c=CN,HK,TW,JP,US"},
		},
	},
	"quanx": {
		Name:  "quanx",
		Title: "QuanX - Free Proxies",
		Icon:  `<i aria-hidden="true" class="metron-quantumultx text-2xl"></i>`,
		Desc:  "Quantumult X 节点订阅",
		Note:  `QuanX 导入方法：打开 Quantumult X，进入 <b>节点 → 订阅</b>，右上角 + 号粘贴订阅地址；或将地址填入 <b>设置 → 资源解析器</b> 的 server 段自动解析。支持 type（类型）、c（国家）、speed、filter、tls、reality 等筛选参数。`,
		NodeRows: []clientPageRow{
			{Label: "全部节点订阅", Rel: "/quanx/proxies"},
			{Label: "按类型订阅", Rel: "/quanx/proxies?type=vless"},
			{Label: "常用国家订阅", Rel: "/quanx/proxies?c=CN,HK,TW,JP,US"},
		},
	},
	"v2rayn": {
		Name:  "v2rayn",
		Title: "v2rayN - Free Proxies",
		Icon:  `<i aria-hidden="true" class="metron-v2rayng text-2xl"></i>`,
		Desc:  "v2rayN 节点订阅",
		Note:  `v2rayN 导入方法：<b>订阅分组 → 订阅设置 → 添加</b>，填入上方订阅地址（URL）后点击「订阅更新」。支持 type（类型）、c（国家）、speed、filter、tls、reality 等筛选参数。`,
		NodeRows: []clientPageRow{
			{Label: "全部节点订阅", Rel: "/v2rayn/proxies"},
			{Label: "按类型订阅", Rel: "/v2rayn/proxies?type=vless"},
			{Label: "常用国家订阅", Rel: "/v2rayn/proxies?c=CN,HK,TW,JP,US"},
		},
	},
	"shadowrocket": {
		Name:  "shadowrocket",
		Title: "ShadowRocket - Free Proxies",
		Icon:  `<i aria-hidden="true" class="metron-shadowrocket text-2xl"></i>`,
		Desc:  "ShadowRocket 节点订阅（Clash 格式）",
		Note:  `ShadowRocket 的订阅包含了 Clash 格式，可以参考 <a href="clash" class="text-indigo-600 dark:text-indigo-400 underline">Clash</a> 进行筛选。`,
		NodeRows: []clientPageRow{
			{Label: "全部节点订阅", Rel: "/clash/proxies"},
			{Label: "常用国家订阅", Rel: "/clash/proxies?c=CN,HK,TW,JP,US"},
		},
	},
}

// renderClientPage 用统一 client.html 模板渲染客户端页（合并 6 个页面文件）
func renderClientPage(c *gin.Context, name string) {
	pg := clientPages[name]
	base := templateBasePath(c)
	domain := config.Config().Domain
	for i := range pg.ConfigRows {
		r := &pg.ConfigRows[i]
		switch {
		case r.Text == "LOCAL":
			r.Text = "https://" + domain + base + "/clash/localconfig"
			r.LinkURL = template.URL("clash://install-config?url=" + url.QueryEscape(r.Text))
		case r.LinkScheme != "":
			r.LinkURL = template.URL(r.LinkScheme + "?url=" + url.QueryEscape("https://"+domain+base+r.Rel))
		}
	}
	c.HTML(http.StatusOK, "client.html", gin.H{
		"domain":       domain,
		"base_path":    base,
		"version":      version,
		"active":       pg.Name,
		"show_tg":      false,
		"client_title": pg.Title,
		"client_name":  pg.Name,
		"client_icon":  pg.Icon,
		"client_desc":  pg.Desc,
		"client_note":  template.HTML(pg.Note),
		"config_rows":  pg.ConfigRows,
		"node_rows":    pg.NodeRows,
	})
}



func SetVersion(v string) {
	version = v
}

// cachedResponse 缓存的 HTTP 响应
//
// 注：gin-contrib/cache 的 SiteCache 只读不写（缓存永远不会被填充，等价于空操作），
// 此处实现一个可用的站点缓存：未命中时包装 ResponseWriter 记录响应，命中时直接回放。
type cachedResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

// cacheWriter 包装 gin.ResponseWriter，拦截响应写入以便缓存
//
// 注：gin-contrib/cache 的 SiteCache 只读不写（缓存永远不会被填充，等价于空操作），
// 此处实现一个可用的站点缓存：未命中时包装 ResponseWriter 记录响应，命中时直接回放。
type cacheWriter struct {
	gin.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *cacheWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *cacheWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *cacheWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// siteCache 站点响应缓存中间件：
// 未命中时缓存 2xx 响应，命中时直接回放；
// 触发类/动态接口（/task/、/health、/link/、/debug/*）通过 skipPrefixes 跳过缓存。
func siteCache(store persistence.CacheStore, expire time.Duration, skipPrefixes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 带 random=true 的请求（/bestProxyIp 随机打乱节点顺序）不缓存，
		// 保证每次调用都重新随机，否则 1 分钟内会返回相同的排列
		if r := c.Query("random"); r == "true" || r == "1" {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		for _, p := range skipPrefixes {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}

		// 用 URL.RequestURI() 重建请求 URI（含 query）作为缓存 key：
		// 不依赖服务端填充的 RequestURI 字段，httptest 等场景也正确区分
		key := cache.CreateKey(c.Request.URL.RequestURI())
		var resp cachedResponse
		if err := store.Get(key, &resp); err == nil {
			// 命中缓存：回放状态码、响应头与响应体
			for k, vs := range resp.Header {
				for _, v := range vs {
					c.Header(k, v)
				}
			}
			c.Status(resp.Status)
			_, _ = c.Writer.Write(resp.Body)
			c.Abort()
			return
		}

		// 未命中：包装 writer 记录响应
		w := &cacheWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = w
		c.Next()

		// 仅缓存成功的完整响应，避免缓存错误页与空响应
		if w.status >= 200 && w.status < 300 && w.body.Len() > 0 {
			_ = store.Set(key, cachedResponse{
				Status: w.status,
				Header: w.Header().Clone(),
				Body:   w.body.Bytes(),
			}, expire)
		}
	}
}

// serveProxyList 统一处理 /clash|/surge|/loon|/v2rayn/proxies 四个接口的逻辑：
// 解析公共筛选参数，无筛选时优先返回缓存，type=all 时使用全量代理。
func serveProxyList(c *gin.Context, cacheKey string, supportUnderlyingProxy bool, provide func(proxy.ProxyList, provider.Base) string) string {
	proxyTypes := c.DefaultQuery("type", "")
	proxyCountry := c.DefaultQuery("c", "")
	proxyNotCountry := c.DefaultQuery("nc", "")
	proxySpeed := c.DefaultQuery("speed", "")
	proxyFilter := c.DefaultQuery("filter", "")
	proxyUnderlyingProxy := c.DefaultQuery("underlyingproxy", "")
	proxyTLS := c.DefaultQuery("tls", "")
	proxyReality := c.DefaultQuery("reality", "")

	noFilter := proxyTypes == "" && proxyCountry == "" && proxyNotCountry == "" &&
		proxySpeed == "" && proxyFilter == "" && proxyTLS == "" && proxyReality == "" &&
		(!supportUnderlyingProxy || proxyUnderlyingProxy == "")

	base := provider.Base{
		Types:           proxyTypes,
		Country:         proxyCountry,
		NotCountry:      proxyNotCountry,
		Speed:           proxySpeed,
		Filter:          proxyFilter,
		UnderlyingProxy: proxyUnderlyingProxy,
		TLS:             proxyTLS,
		Reality:         proxyReality,
	}

	if noFilter {
		// 命中缓存则直接返回；否则生成一次并缓存（测速后由任务刷新）
		if text := appcache.GetString(cacheKey); text != "" {
			return text
		}
		proxies := appcache.GetProxies("proxies")
		base.Proxies = &proxies
		text := provide(proxies, base)
		appcache.SetString(cacheKey, text)
		return text
	}

	// 根据Query筛选节点：type=all 使用全量节点，否则使用可用节点
	key := "proxies"
	if proxyTypes == "all" {
		key = "allproxies"
	}
	proxies := appcache.GetProxies(key)
	base.Proxies = &proxies
	return provide(proxies, base)
}

// subHandler 生成 /ss|/ssr|/vmess|/sip002|/trojan|/vless/sub 接口 handler，
// 支持 tls / reality 过滤参数（与 /proxies 接口一致）
func subHandler(types string, provide func(provider.Base) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		proxies := appcache.GetProxies("proxies")
		base := provider.Base{
			Proxies: &proxies,
			Types:   types,
			TLS:     c.Query("tls"),
			Reality: c.Query("reality"),
		}
		c.String(http.StatusOK, provide(base))
	}
}

// bestIPHandler 统一 /best* 接口的配置加载与错误处理。
// config.Parse 带 mtime 缓存，未变化时零开销。
func bestIPHandler(fn func(c *gin.Context) (string, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := config.Parse(""); err != nil {
			log.Errorln("config parse error: %s", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		// 优选订阅鉴权：best 节点链接基于 proxy_info（自用 VPS 凭据）生成，
		// 配置 best_token 后必须携带 ?token=xxx（未配置保持公开，兼容旧部署）。
		if token := config.Config().BestToken; token != "" && c.Query("token") != token {
			c.String(http.StatusUnauthorized, "unauthorized")
			return
		}
		text, err := fn(c)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.String(http.StatusOK, text)
	}
}

// parseBestIPParams 解析 /best* 公共路径与查询参数
func parseBestIPParams(c *gin.Context) (format, distCountry string) {
	format = c.Param("format")
	distCountry = c.Query("d")
	if distCountry == "" {
		distCountry = "JP"
	}
	return
}

// isTrue 解析 "true"/"1" 布尔查询参数
func isTrue(v string) bool {
	return v == "true" || v == "1"
}

// parseIPV6Mode 解析 ipv6 查询参数为三态：
// 空/未传 → 0（IPv4+IPv6 都输出）；true/1 → 1（仅 IPv6）；false/0 → 2（仅 IPv4）
func parseIPV6Mode(c *gin.Context) int {
	switch strings.ToLower(c.Query("ipv6")) {
	case "true", "1":
		return 1
	case "false", "0":
		return 2
	default:
		return 0
	}
}

func setupRouter() {
	gin.SetMode(gin.ReleaseMode)
	router = gin.New()              // 没有任何中间件的路由
	temp, err := loadHTMLTemplate() // 加载html模板，模板源存放于html.go中的类似_assetsHtmlSurgeHtml的变量
	if err != nil {
		panic(err)
	}
	router.SetHTMLTemplate(temp) // 应用模板

	store := persistence.NewInMemoryStore(time.Minute)
	// 站点响应缓存；触发类/动态接口跳过（/task/*、/health、/link/、/debug/*）
	router.Use(gin.Recovery(), siteCache(store, time.Minute,
		"/task/", "/health", "/link/", "/debug/statsviz", "/debug/pprof"))

	pprof.Register(router)

	// Create statsviz server.
	srv, _ := statsviz.NewServer(statsviz.Root("/debug/statsviz"))
	ws := srv.Ws()
	index := srv.Index()

	router.GET("/debug/statsviz/*filepath", func(context *gin.Context) {
		if context.Param("filepath") == "/ws" {
			ws(context.Writer, context.Request)
			return
		}
		index(context.Writer, context.Request)
	})

	// 静态资源（本地化 CSS/JS/字体，无外部 CDN 依赖）
	if staticSub, err := fs.Sub(config.StaticFS, "assets/static"); err == nil {
		router.StaticFS("/static", http.FS(staticSub))
	}

	router.GET("/", func(c *gin.Context) {
		bestNodes := appcache.GetBestNodeList("bestNode")
		bestTotal, bestHealthy, bestAnyTLS := 0, 0, 0
		for _, n := range bestNodes {
			bestTotal++
			if n.Healthy {
				bestHealthy++
			}
			if n.AnyTLS {
				bestAnyTLS++
			}
		}
		c.HTML(http.StatusOK, "index.html", gin.H{
			"domain":                      config.Config().Domain,
			"getters_count":               appcache.GettersCount,
			"all_proxies_count":           appcache.AllProxiesCount,
			"ss_proxies_count":            appcache.SSProxiesCount,
			"ssr_proxies_count":           appcache.SSRProxiesCount,
			"vmess_proxies_count":         appcache.VmessProxiesCount,
			"trojan_proxies_count":        appcache.TrojanProxiesCount,
			"vless_proxies_count":         appcache.VlessProxiesCount,
			"useful_proxies_count":        appcache.UsefullProxiesCount,
			"useful_ss_proxies_count":     appcache.UsefullSSProxiesCount,
			"useful_ssr_proxies_count":    appcache.UsefullSSRProxiesCount,
			"useful_vmess_proxies_count":  appcache.UsefullVmessProxiesCount,
			"useful_trojan_proxies_count": appcache.UsefullTrojanProxiesCount,
			"useful_vless_proxies_count":  appcache.UsefullVlessProxiesCount,
			"anytls_proxies_count":        appcache.AnyTLSProxiesCount,
			"useful_anytls_proxies_count": appcache.UsefullAnyTLSProxiesCount,
			"best_total":                  bestTotal,
			"best_healthy":                bestHealthy,
			"best_anytls":                 bestAnyTLS,
			"best_v6_healthy":             app.CountBestV6Healthy(),
			"best_last_update":            appcache.GetString("bestNodeLastUpdateTime"),
			"last_crawl_time":             appcache.LastCrawlTime,
			"is_speed_test":               appcache.IsSpeedTest,
			"version":                     version,
			"geo_ip_db_version":           geoIp.GeoIpDBCurVersion,
		})
	})

	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	router.GET("/clash", func(c *gin.Context) {
		renderClientPage(c, "clash")
	})
	router.GET("/surge", func(c *gin.Context) {
		renderClientPage(c, "surge")
	})
	router.GET("/shadowrocket", func(c *gin.Context) {
		renderClientPage(c, "shadowrocket")
	})
	router.GET("/loon", func(c *gin.Context) {
		renderClientPage(c, "loon")
	})
	router.GET("/quanx", func(c *gin.Context) {
		renderClientPage(c, "quanx")
	})
	router.GET("/v2rayn", func(c *gin.Context) {
		renderClientPage(c, "v2rayn")
	})

	// 优选 IP 订阅说明页
	router.GET("/best", func(c *gin.Context) {
		cp, _ := json.Marshal(config.Config().ProxyInfo.CountryProtocols())
		cpc, _ := json.Marshal(clientProtocols)
		c.HTML(http.StatusOK, "best.html", gin.H{
			"domain":            config.Config().Domain,
			"base_path":         templateBasePath(c),
			"countries":         config.Config().ProxyInfo.Countries(),
			"country_protocols": template.JS(cp),
			"client_protocols":  template.JS(cpc),
		})
	})

	router.GET("/clash/config", func(c *gin.Context) {
		c.HTML(http.StatusOK, "clash-config.yaml", gin.H{
			"domain": config.Config().Domain,
			"base_path": templateBasePath(c),
		})
	})
	router.GET("/clash/localconfig", func(c *gin.Context) {
		c.HTML(http.StatusOK, "clash-config-local.yaml", gin.H{
			"port": config.Config().Port,
		})
	})

	router.GET("/surge/config", func(c *gin.Context) {
		c.HTML(http.StatusOK, "surge.conf", gin.H{
			"domain": config.Config().Domain,
			"base_path": templateBasePath(c),
		})
	})

	router.GET("/clash/proxies", func(c *gin.Context) {
		text := serveProxyList(c, "clashproxies", false, func(proxies proxy.ProxyList, base provider.Base) string {
			return provider.Clash{Base: base}.Provide()
		})
		c.String(http.StatusOK, text)
	})
	router.GET("/surge/proxies", func(c *gin.Context) {
		text := serveProxyList(c, "surgeproxies", true, func(proxies proxy.ProxyList, base provider.Base) string {
			return provider.Surge{Base: base}.Provide()
		})
		c.String(http.StatusOK, text)
	})

	router.GET("/loon/proxies", func(c *gin.Context) {
		text := serveProxyList(c, "loonproxies", false, func(proxies proxy.ProxyList, base provider.Base) string {
			return provider.Loon{Base: base}.Provide()
		})
		c.String(http.StatusOK, text)
	})

	router.GET("/v2rayn/proxies", func(c *gin.Context) {
		text := serveProxyList(c, "v2raynproxies", false, func(proxies proxy.ProxyList, base provider.Base) string {
			return provider.V2rayn{Base: base}.Provide()
		})
		c.String(http.StatusOK, text)
	})

	router.GET("/quanx/proxies", func(c *gin.Context) {
		text := serveProxyList(c, "quanxproxies", false, func(proxies proxy.ProxyList, base provider.Base) string {
			return provider.QuanX{Base: base}.Provide()
		})
		c.String(http.StatusOK, text)
	})

	router.GET("/ss/sub", subHandler("ss", func(b provider.Base) string { return provider.SSSub{Base: b}.Provide() }))
	router.GET("/ssr/sub", subHandler("ssr", func(b provider.Base) string { return provider.SSRSub{Base: b}.Provide() }))
	router.GET("/vmess/sub", subHandler("vmess", func(b provider.Base) string { return provider.VmessSub{Base: b}.Provide() }))
	router.GET("/sip002/sub", subHandler("ss", func(b provider.Base) string { return provider.SIP002Sub{Base: b}.Provide() }))
	router.GET("/trojan/sub", subHandler("trojan", func(b provider.Base) string { return provider.TrojanSub{Base: b}.Provide() }))
	router.GET("/vless/sub", subHandler("vless", func(b provider.Base) string { return provider.VlessSub{Base: b}.Provide() }))

	router.GET("/link/:id", func(c *gin.Context) {
		proxies := appcache.GetProxies("allproxies")
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		if id >= proxies.Len() || id < 0 {
			c.String(http.StatusInternalServerError, "id out of range")
			return
		}
		c.String(http.StatusOK, proxies[id].Link())
	})

	// 任务接口：后台执行，任务实现（含互斥防并发与 GC）收敛在 internal/app。
	// 配置 admin_token 后需携带 ?token=xxx 才可触发（未配置则保持公开，兼容旧部署）。
	taskHandler := func(fn func()) gin.HandlerFunc {
		return func(c *gin.Context) {
			if token := config.Config().AdminToken; token != "" && c.Query("token") != token {
				c.String(http.StatusUnauthorized, "unauthorized")
				return
			}
			go fn()
			c.String(http.StatusOK, "ok")
		}
	}
	router.GET("/task/crawl", taskHandler(app.CrawlTask))
	router.GET("/task/speedtest", taskHandler(app.SpeedTestTask))
	router.GET("/task/updateGeoIP", taskHandler(app.GeoIPTask))
	router.GET("/task/updateBestNode", taskHandler(app.BestNodeTask))

	router.GET("/bestProxyIp/:format", bestIPHandler(func(c *gin.Context) (string, error) {
		format, distCountry := parseBestIPParams(c)
		limit := 0
		if s := c.Query("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil {
				return "", errors.New("invalid limit parameter")
			}
			limit = n
		}
		return app.SubNiceProxyIp(format, distCountry, c.Query("c"), limit, isTrue(c.Query("random")), c.Query("sort"), parseIPV6Mode(c), c.Query("cdn"))
	}))

	router.GET("/bestCfProxyIp/:format", bestIPHandler(func(c *gin.Context) (string, error) {
		format, distCountry := parseBestIPParams(c)
		return app.SubNiceCfProxyIp(format, distCountry, parseIPV6Mode(c))
	}))

	router.GET("/bestCfProxyDomainTop20/:format", bestIPHandler(func(c *gin.Context) (string, error) {
		format, distCountry := parseBestIPParams(c)
		return app.SubNiceCfProxyIpTop20(format, distCountry, isTrue(c.Query("ips")), parseIPV6Mode(c))
	}))

	router.GET("/bestCfProxyIpTop20/:format", bestIPHandler(func(c *gin.Context) (string, error) {
		format, distCountry := parseBestIPParams(c)
		return app.SubNiceCfProxyIpTop20(format, distCountry, true, parseIPV6Mode(c))
	}))

	router.GET("/bestCfProxyIpIsp/:format", bestIPHandler(func(c *gin.Context) (string, error) {
		format, distCountry := parseBestIPParams(c)
		return app.SubNiceCfProxyIpProvider(format, c.Query("isp"), distCountry, parseIPV6Mode(c))
	}))

	router.GET("/bestCfProxySub/:format", bestIPHandler(func(c *gin.Context) (string, error) {
		format, distCountry := parseBestIPParams(c)
		return app.SubNiceCfProxySub(format, c.Query("sub"), distCountry, parseIPV6Mode(c))
	}))

	router.GET("/bestIpKr/:format", bestIPHandler(func(c *gin.Context) (string, error) {
		format, _ := parseBestIPParams(c)
		limit := 0
		if s := c.Query("limit"); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil {
				return "", errors.New("invalid limit parameter")
			}
			limit = n
		}
		return app.SubNiceProxyIp(format, "KR", c.Query("c"), limit, isTrue(c.Query("random")), c.Query("sort"), parseIPV6Mode(c), c.Query("cdn"))
	}))
}


// templateBasePath 模板渲染用的部署前缀（注入前端 window.PROXYPOOL_BASE_PATH 用）：
// 配置 base_path > X-Forwarded-Prefix 头 > X-Forwarded-URI/X-Original-URL 首段。
// 返回不带尾斜杠的前缀（如 "/show"），空串表示根路径部署。
// 订阅链接的前缀最终由前端从 location 自算兜底，此处仅尽力而为。
func templateBasePath(c *gin.Context) string {
	if bp := config.Config().BasePath; bp != "" {
		return strings.TrimSuffix(bp, "/")
	}
	if bp := c.GetHeader("X-Forwarded-Prefix"); bp != "" {
		return strings.TrimSuffix(bp, "/")
	}
	for _, h := range []string{"X-Forwarded-URI", "X-Original-URL"} {
		if u := c.GetHeader(h); u != "" && strings.HasPrefix(u, "/") {
			if i := strings.Index(u[1:], "/"); i > 0 {
				return u[:i+1]
			}
			return ""
		}
	}
	return ""
}

// applyBasePath 剥离部署前缀并规范化路径（确保以 / 开头）。
// 供 serveHandler 与单元测试复用。
func applyBasePath(path, basePath string) string {
	if basePath != "" && strings.HasPrefix(path, basePath) {
		path = strings.TrimPrefix(path, basePath)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
	}
	return path
}

// knownRoutePrefixes 注册路由的静态前缀（供子路径自动推断使用，setupRouter 后构建）。
// 例如 "/bestProxyIp/:format" → "/bestProxyIp/"，"/static/*filepath" → "/static/"，"/task/crawl" → "/task/crawl"。
var knownRoutePrefixes []string

// buildKnownRoutePrefixes 从已注册路由收集静态前缀。
func buildKnownRoutePrefixes() {
	seen := make(map[string]struct{})
	for _, r := range router.Routes() {
		p := r.Path
		if p == "/" {
			continue // 首页兜底，不作为前缀识别依据
		}
		// 截断动态参数（:param / *filepath）之前的静态部分
		static := p
		for i := 0; i < len(p); i++ {
			if p[i] == ':' || p[i] == '*' {
				// 取最后一个 / 之前（含 /），保证前缀语义
				if j := strings.LastIndex(p[:i], "/"); j >= 0 {
					static = p[:j+1]
				} else {
					static = "/"
				}
				break
			}
		}
		if _, ok := seen[static]; !ok {
			seen[static] = struct{}{}
			knownRoutePrefixes = append(knownRoutePrefixes, static)
		}
	}
}

// matchKnownRoute 判断 path 是否命中某个已知路由前缀（等于或以 "xxx/" 前缀开始）。
func matchKnownRoute(path string) bool {
	for _, p := range knownRoutePrefixes {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(path, p) {
				return true
			}
		} else if path == p {
			return true
		}
	}
	return false
}

func Run() {
	setupRouter()
	buildKnownRoutePrefixes()
	servePort := config.Config().Port
	// heroku 平台通过 PORT 环境变量指定端口（仅 heroku 生效，避免普通部署被环境变量覆盖）
	if envp := os.Getenv("PORT"); envp != "" && os.Getenv("DYNO") != "" {
		servePort = envp
	}

	// 部署子路径支持（自动适配，base_path 非必填，支持多个前缀）：
	//  1) 显式配置 base_path → 直接使用
	//  2) 反向代理设置 X-Forwarded-Prefix 头 → 使用（nginx 可配 proxy_set_header X-Forwarded-Prefix /show/）
	//  3) 自动推断：请求路径中识别出已知路由的静态前缀，其前方即部署前缀；
	//     支持同时存在多个前缀（如 /show/ 与 /proxy/ 都转发到本服务），识别后缓存
	var (
		prefixesMu       sync.Mutex
		detectedPrefixes = make(map[string]struct{})
	)
	resolveBasePath := func(r *http.Request) string {
		if bp := config.Config().BasePath; bp != "" {
			return bp
		}
		if bp := r.Header.Get("X-Forwarded-Prefix"); bp != "" {
			return bp
		}
		p := r.URL.Path
		prefixesMu.Lock()
		defer prefixesMu.Unlock()
		// 1) 命中已缓存前缀（最长优先，避免 /a/ 误匹配 /a/b/）
		best := ""
		for prefix := range detectedPrefixes {
			if strings.HasPrefix(p, prefix) && len(prefix) > len(best) {
				best = prefix
			}
		}
		if best != "" {
			return best
		}
		// 2) 根路径路由，无前缀
		if matchKnownRoute(p) {
			return ""
		}
		// 3) 尝试推断新前缀："/show/clash" → 候选 "/show/"，剩余 "/clash" 命中已知路由
		if i := strings.Index(p[1:], "/"); i >= 0 {
			prefix := p[:i+1] + "/"
			if matchKnownRoute(p[i+1:]) {
				detectedPrefixes[prefix] = struct{}{}
				return prefix
			}
		}
		return ""
	}

	serveHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		basePath := resolveBasePath(r)
		r.URL.Path = applyBasePath(r.URL.Path, basePath)
		router.ServeHTTP(w, r)
	})

	// Run on this server
	var err error
	if config.Config().TLSEnable {
		err = http.ListenAndServeTLS(":"+servePort, config.Config().CertFile, config.Config().KeyFile, serveHandler)
	} else {
		err = http.ListenAndServe(":"+servePort, serveHandler)
	}

	if err != nil {
		log.Errorln("router: Web server starting failed. Make sure your port %s has not been used. \n%s", servePort, err.Error())
	} else {
		log.Infoln("Proxypool is serving on port: %s", servePort)
	}
}

// 返回页面templates
func loadHTMLTemplate() (t *template.Template, err error) {
	t, err = template.New("").ParseFS(config.HtmlFs, "assets/html/*")
	return
}
