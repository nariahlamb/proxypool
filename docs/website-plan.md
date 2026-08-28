# 网站 HTML 功能完善方案

## 一、现状诊断（config/assets/html/index.html + static/index.js）

### 功能缺失
| # | 问题 | 现状 |
|---|------|------|
| 1 | **无优选 IP（best）入口** | 项目核心新功能 7 个 `/best*` 端点在首页完全不可见 |
| 2 | **无 best 统计** | 统计区只展示普通节点数，best 节点/健康数/透传数无展示 |
| 3 | **anytls 订阅链接不准确** | `https://{{domain}}/clash/proxies?type=anytls` 指向常规节点过滤，未指向 best 的 `xxxAnytls` 订阅 |
| 4 | **无 anytls / best 说明页** | best 端点有格式+参数（country/limit/random/cdn/ipv6）但无文档页 |

### 过时 / 错误
| # | 问题 | 现状 |
|---|------|------|
| 5 | 外部 CDN 依赖 | CSS 引用 `cdn.jsdelivr.net/gh/sansui233/proxypool-fe/...`、jQuery 引用 `bootcdn`——**CDN 挂则页面崩**；且项目已改名 One-Piecs，CDN 仍指向旧 fork |
| 6 | footer 版权/推广过时 | 链接 zu1k/sansui233 旧仓库，无当前仓库；"More" 跳第三方 bianyuan.xyz（旧推广位） |
| 7 | 语言/可访问性 | `lang="en"` 但内容全中文；`user-scalable=0` 禁缩放 |
| 8 | meta/keywords 过时 | 描述未含 vless/anytls/优选IP |
| 9 | 无运维信息 | 任务触发（/task/*）、statsviz、冻结节点数未展示 |

## 二、修改方案

### A. 后端小改（数据支撑）

| 文件 | 改动 |
|------|------|
| `api/router.go` | 首页模板渲染时计算并传入 best 统计：`best_total` / `best_healthy` / `best_anytls` / `best_last_update`（从 `cache.GetBestNodeList` 现算，无需新缓存变量） |
| `api/router.go` | 新增 `/best` 路由：渲染说明页 `best.html` |
| `internal/cache/cache.go` | （可选）增加 `GetBestStats()` 便捷方法：总数/Healthy 数/AnyTLS 数 |

### B. 首页 index.html 改造

1. **导航栏**：新增「优选IP」入口（链接 `/best`），保留 Clash/Shadowrocket/Surge/Loon/QuanX
2. **统计区**：追加 best 统计行：优选节点 `{{best_total}}`、健康 `{{best_healthy}}`、anytls 透传 `{{best_anytls}}`、更新时间 `{{best_last_update}}`
3. **新增「优选 IP 订阅」区块**：7 个端点表格（bestProxyIp/bestCfProxyIp/bestCfProxyIpTop20/bestCfProxyIpIsp/bestCfProxyDomainTop20/bestCfProxySub/bestIpKr）+ 格式示例（`clashVmess`/`surgeTrojan`/`loonAnytls` 等）+ 参数说明（country/limit/random/cdn/ipv6）+ 复制按钮
4. **订阅链接表**：修正 anytls 行为 best 订阅（`/bestCfProxyIp/clashAnytls`），补充 best 订阅行
5. **footer**：更新仓库链接为 One-Piecs/proxypool（保留 zu1k/sansui233 致谢），移除 bianyuan.xyz 推广位
6. **元信息**：`lang="zh-CN"`、更新 title/meta/keywords、移除禁缩放
7. （可选）任务区：`/task/crawl`、`/task/speedtest`、`/task/updateBestNode` 触发按钮

### C. 新增 best 说明页 `config/assets/html/best.html`

- 7 个 best 端点 + 参数说明 + 各客户端格式示例（含复制按钮）
- 说明 bestip_probe（vless 健康检查）与 sni_probe（anytls 透传）的过滤语义：
  - vless/vmess/trojan 格式仅导出 Healthy 节点（bestip_probe 启用时）
  - xxxAnytls 仅导出 AnyTLS 节点（sni_probe 启用时）

### D. 前端资源本地化（消除 CDN 依赖）

| 文件 | 改动 |
|------|------|
| `config/assets/static/index.js` | 用**原生 JS 重写**（去 jQuery）：navbar burger 切换 + 复制功能（navigator.clipboard 兜底 execCommand），删除对 bootcdn jQuery 的依赖 |
| `config/assets/static/site.css` | 下载/内联 sansui233/proxypool-fe 的 index.css + metron 图标字体到本地 embed |
| `config/assets/html/*.html` | 全部改引本地 `/static/site.css`、`/static/index.js` |
| `config/assets.go` | embed 新增 `assets/static/site.css`（已有 StaticFS 通配） |

### E. 风险与兼容
- HTML 为 go:embed + 服务端模板渲染，改 HTML 后需重新编译生效（Docker 构建自动包含）
- 模板变量新增不影响旧字段，向后兼容
- CSS 本地化需确认 metron 图标字体体积（约几十 KB，可接受）
- `/best` 说明页为静态页，无后端逻辑风险

## 三、涉及文件清单

```
config/assets/html/index.html     改造
config/assets/html/best.html      新增
config/assets/static/index.js     重写（原生 JS）
config/assets/static/site.css     新增（本地化样式）
config/assets/html/clash.html 等  改 CSS 引用（可选，若统一）
api/router.go                     模板传参 + /best 路由
internal/cache/cache.go           可选 GetBestStats()
```

## 四、验证

- Docker 内 `go build` + 启动，浏览器检查首页/best 页渲染、复制按钮、无外部 CDN 请求
- 模板渲染测试（api/router_test.go 扩展）
