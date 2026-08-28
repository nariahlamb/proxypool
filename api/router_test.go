package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/One-Piecs/proxypool/config"
	appcache "github.com/One-Piecs/proxypool/internal/cache"
	"github.com/gin-gonic/gin"
)

// TestApplyBasePath 剥离逻辑：单前缀/多前缀/无前缀/空路径规范化
func TestApplyBasePath(t *testing.T) {
	cases := []struct {
		path, base, want string
	}{
		{"/show/clash", "/show/", "/clash"},
		{"/show/", "/show/", "/"},
		{"/show", "/show", "/"},        // 无尾斜杠前缀
		{"/proxy/best", "/proxy/", "/best"},
		{"/clash", "/show/", "/clash"}, // 非前缀路径原样
		{"/", "/show/", "/"},
	}
	for _, c := range cases {
		if got := applyBasePath(c.path, c.base); got != c.want {
			t.Errorf("applyBasePath(%q, %q) = %q, want %q", c.path, c.base, got, c.want)
		}
	}
}

// setupRouterForTest 初始化路由与配置
func setupRouterForTest(t *testing.T, cfgYAML string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if cfgYAML != "" {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(cfgYAML), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := config.Parse(path); err != nil {
			t.Fatalf("config parse: %v", err)
		}
	}
	setupRouter()
}

// TestPageRoutes 核心页面与静态资源可达
func TestPageRoutes(t *testing.T) {
	setupRouterForTest(t, "domain: example.com\n")
	paths := []string{"/", "/clash", "/surge", "/shadowrocket", "/loon", "/quanx", "/v2rayn", "/best"}
	for _, p := range paths {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, p, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", p, w.Code)
		}
	}
	// 静态资源（本地化 CSS/JS）
	for _, p := range []string{"/static/css/index.css", "/static/index.js"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, p, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", p, w.Code)
		}
	}
	// 订阅端点；/best* 无数据时返回 500（设计行为），仅验证路由存在（非 404）
	for _, p := range []string{"/ss/sub", "/clash/proxies"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, p, nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", p, w.Code)
		}
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/bestCfProxyIp/clashVmess", nil)
	router.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Error("/bestCfProxyIp/clashVmess 路由不存在")
	}
}

// TestTaskAuthAdminToken 配置 admin_token 后任务接口鉴权
func TestTaskAuthAdminToken(t *testing.T) {
	setupRouterForTest(t, "admin_token: \"s3cret\"\n")

	// 无 token → 401
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/task/crawl", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no token status = %d, want 401", w.Code)
	}

	// 错误 token → 401
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/task/crawl?token=wrong", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong token status = %d, want 401", w.Code)
	}

	// 正确 token → 200
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/task/crawl?token=s3cret", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("right token status = %d, want 200", w.Code)
	}
}

// TestTaskPublicNoToken 未配置 admin_token 时任务接口公开
func TestTaskPublicNoToken(t *testing.T) {
	setupRouterForTest(t, "domain: example.com\n")
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/task/crawl", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("public task status = %d, want 200", w.Code)
	}
}

// TestBestAuthToken 配置 best_token 后 /best* 接口鉴权（自用 VPS 凭据防泄露）
func TestBestAuthToken(t *testing.T) {
	setupRouterForTest(t, "best_token: \"s3cret\"\nproxy_info:\n  JP:\n    vless:\n      host: \"example.com\"\n      uuid: \"u\"\n      path: \"/p\"\n")
	// 注入测试 best 节点（无采集环境）
	appcache.SetBestNodeList("bestNode", []appcache.BestNode{{Ip: "1.2.3.4", Port: 443, Country: "JP"}})

	// 无 token → 401
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/bestProxyIp/clashVless?c=JP", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no token status = %d, want 401", w.Code)
	}

	// 错误 token → 401
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/bestProxyIp/clashVless?c=JP&token=wrong", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong token status = %d, want 401", w.Code)
	}

	// 正确 token → 200（配置 proxy_info 提供 JP 节点模板）
	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/bestProxyIp/clashVless?c=JP&token=s3cret", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("right token status = %d, want 200", w.Code)
	}
}

// TestBestPublicNoToken 未配置 best_token 时 /best* 接口公开（兼容旧部署）
func TestBestPublicNoToken(t *testing.T) {
	setupRouterForTest(t, "domain: example.com\nproxy_info:\n  JP:\n    vless:\n      host: \"example.com\"\n      uuid: \"u\"\n      path: \"/p\"\n")
	appcache.SetBestNodeList("bestNode", []appcache.BestNode{{Ip: "1.2.3.4", Port: 443, Country: "JP"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/bestProxyIp/clashVless?c=JP", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("public best status = %d, want 200", w.Code)
	}
}
