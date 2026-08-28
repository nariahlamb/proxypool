## [v2.5.16] - 2026-08-29

### 🌐 best 订阅各客户端 IPv6 格式修正

- 规则：host:port 连写的格式需方括号（URL/v2rayN、节点名、QuanX 的 addr:port）；
  端口为独立字段的 server 用裸 IPv6（Surge/Loon/Clash）
- 修复 Surge server 字段误加方括号；补 Loon 6 个生成器 + Clash anytls 的 server 裸地址
- 新增 stripBrackets helper + 多客户端格式断言单测（Surge/Loon/Clash 裸、QuanX/v2rayN 方括号）

## [v2.5.15] - 2026-08-29

### 🌐 docker-compose 示例改用 host 网络（IPv6 出口可用）

- 容器默认 bridge 网络无 IPv6（Docker 不做 IPv6 NAT），宿主机有 IPv6 容器也出不了公网
- host 网络直接共享宿主机网络栈：IPv4+IPv6 出口均可用，IPv6 优选节点健康检查正常
- 保留 bridge（仅 IPv4）注释选项

### 🔧 best 探测 IPv6 出口检测 + 日志收敛

- 探测前检测本机 IPv6 出口：无出口时跳过 IPv6 节点（一条汇总日志），
  修复数百条 network is unreachable 失败日志刷屏
- 移除失败日志中的 vless:// 链接（含 proxy_info 凭据，避免泄入日志），
  保留无凭据的 Debug 级错误记录

### 🐛 best 明文订阅解析噪音过滤

- 订阅源返回 HTML/JS 错误页时，JS 代码行被误当节点解析（xxx:443 子串）
- isValidHostname 校验 host（拒绝脚本特征字符）+ URL 行仅接受已知协议前缀
- DNS 解析失败日志降级为 Debug（免费源废弃域名属常态噪音）

## [v2.5.14] - 2026-08-29

### 🐛 修复 best 健康检查 IPv6 节点全部失败（mihomo DisableIPv6 默认值）

- 详细日志排查发现失败原因为 mihomo `dns resolve failed: ip version error`：
  mihomo 默认 DisableIPv6=true（resolver 只解析 IPv4，IPv6 字面地址被拒）——
  即使服务器有 IPv6 出口，IPv6 节点探测也必然失败
- 修复：healthcheck init 显式 `resolver.DisableIPv6 = false`
- 实测验证（o.laibas.top 凭据 + http://cp.cloudflare.com/generate_204）：
  IPv6 健康 0/26 → 14/14（100%）；IPv4 70/72（97%）
- IPv6 节点失败时打印详细诊断（完整 vless 链接 + 错误，便于排查）
- 默认探测 URL 恢复 http://cp.cloudflare.com/generate_204（实测 204）

## [v2.5.13] - 2026-08-28

### 🔧 sub_ip_url 解析健壮性增强

- base64 解码前 TrimSpace：部分源尾部带换行/空白导致严格解码失败整源被丢弃
- 解码失败回退明文解析：vless:// URL 行（缺端口补 443 + 白名单校验）、host:port#注释 行；
  过滤 HTML/JSON 错误页噪音行
- 日志脱敏（maskURLHost）；源返回空/错误页时退避重试，不再本周期永久跳过

## [v2.5.12] - 2026-08-28

### 📊 首页优选 IP 统计增加「IPv6 可用」

- best 健康检查后新增可用 IPv6 节点数统计（Healthy 且 IPv6），首页展示：
  优选 IP：节点 N 个，健康 N，IPv6 可用 N，anytls 透传 N
- CountBestV6Healthy + 单测（IPv4/健康/不健康/未探测区分）

## [v2.5.11] - 2026-08-28

### 🐛 紧急修复：首页加载被 best_token prompt 拦截

- 首页 index.html 含多个 /best* 订阅链接，getSubURL 在页面渲染阶段就对 /best*
  链接弹 prompt 要求输入 best_token → 配置 best_token 后打开首页即弹窗（误以为需要鉴权）
- 修复：渲染阶段只拼已保存的 token（不弹窗）；仅用户主动点击「复制」best 订阅时
  才由 ensureBestToken 提示输入一次并存储
- JS 行为验证：渲染不弹窗/不拼 token，复制拼 token 且只弹一次，普通订阅不受影响

## [v2.5.10] - 2026-08-28

### 🌐 best 订阅 ipv6 参数改为三态

- 默认（不带 ipv6 参数）：IPv4 + IPv6 都输出
- ipv6=true：仅输出 IPv6（补方括号 [addr]）
- ipv6=false：仅输出 IPv4
- 此前默认仅输出 IPv4（双向过滤的回归修正）
- matchIPV6Mode 三态过滤 + SubNiceProxyIp 集成测试（注入 IPv4/IPv6 节点断言三态输出）

### 🐛 best 订阅 IPv6 输出修复

- 源（如 steep.laibas.top/sub）含 IPv6 节点（54 个中 15 个），解析链路原本正常，
  但输出端缺陷：默认时 IPv6 节点也输出且缺方括号 → 生成坏链接
- 双向过滤：ipv6=true 仅输出 IPv6；默认仅输出 IPv4（不影响既有订阅）
- IPv6 输出补方括号：vless://uuid@[2001:db8::1]:443
- 单测：formatNodeHost 方括号 + 双向过滤 + 真实源 IPv6 解析链路验证

## [v2.5.9] - 2026-08-28

### 🔒 best 优选订阅接口鉴权（自用 VPS 凭据防泄露）

- 风险：best 节点链接基于 proxy_info（自用 VPS 的 host/uuid/password/path）生成，
  /best* 接口此前无鉴权公开输出完整可用节点链接，攻击者可免密直连自用 VPS
- 新增 `best_token` 配置：配置后所有 /best* 接口需 ?token=xxx 否则 401；
  未配置保持公开（兼容旧部署），建议公网部署务必配置
- 前端 best 订阅链接复制/渲染时自动拼 token（localStorage 输入一次）
- 日志脱敏：sub_ip_url 只打印 host，隐藏 uuid/password 防泄入日志
- 路由测试补 best 鉴权 4 用例（含公开兼容 + proxy_info 场景）

## [v2.5.8] - 2026-08-28

### 🌐 Surge / Loon / QuanX 页面对齐 v2rayN

### 🌐 Surge / Loon / QuanX 页面对齐 v2rayN

- 三客户端页面重写为 v2rayn.html 风格：统一导航（无 ./ 前缀，适配子路径）、
  全部/按类型/常用国家三行订阅 + 复制按钮、导入方法说明、清理过时版权与 gtag 注释
- Surge 保留「配置文件」区块（/surge/config + surge3 一键导入深链）

### 🤖 OpenAI/ChatGPT 检测精度升级

- 新增状态码判定：api.openai.com 未鉴权 401 = 未封、403 = 封禁（真实 API 探测，
  取代仅凭 text/plain 正文的粗糙判断，解决数据中心 IP 误报可用）
- 网页类 200+HTML 特征 = 可访问；403（text/plain 或 Cloudflare 拦截页）= 封禁
- loc 国家白名单降级为兜底（主判据无法定论时才启用）
- judgeOpenAIResponse 纯函数 + 15 用例单测

## [v2.5.7] - 2026-08-28

### 🐛 启动可用节点统计修正

### 🐛 启动可用节点统计修正

- 启动时"可用节点数量"误显示为库内全部节点（v2.4.1 修复的回归）：
  GetAllProxies 返回全部节点且统一置 useable=false，用其 Len() 统计全量当可用
- 新增 `CountUseableStats()`：按 useable=true 统计总数与类型分布，
  启动首页"可用节点"显示上次真实可用数，"全部节点"仍为库内全量

### 🎨 日志分级着色

### 🎨 日志分级着色

- 日志 LEVEL 按级别着色（终端输出时）：ERROR 红 / WARN 黄 / INFO 绿 / DEBUG 蓝；
  管道/文件/NO_COLOR 环境变量时自动无色（不影响日志收集）

## [v2.5.6] - 2026-08-28

### 🔧 GeoIP 数据库改为运行时下载（镜像不内置）

- 采纳运行时下载设计：Dockerfile 不再 COPY mmdb（构建上下文无需 GeoIP 文件，
  CI 构建不再依赖这些文件存在）；首次启动自动下载 Country.mmdb / ASN / version
- `InitGeoIpDB` 失败降级运行（Warn 继续，不再 os.Exit(1) 崩溃重启），
  可通过 /task/updateGeoIP 重试下载

## [v2.5.5] - 2026-08-28

### 🐛 修复 CI 镜像构建失败（构建上下文缺 GeoIP 数据库）

- GeoIP 数据库（Country.mmdb / GeoLite2-ASN.mmdb / version）原被 .gitignore 排除、
  不在 git 仓库 → CI checkout 后 Dockerfile.release 的 COPY 失败（"assets/version not found"）
- 改为**纳入版本控制**（+20MB），任何构建（本地/CI）均可靠；首次缺失时仍会自动下载

## [v2.5.4] - 2026-08-28

### 🐛 修复 CI 构建失败（go vet copylocks）

- `log/pretty.go` WithAttrs 复制含 sync.Mutex 的 handler 触发 go vet copylocks 检查，
  mutex 改为指针修复；v2.5.2/v2.5.3 的 CI 构建因此失败

## [v2.5.3] - 2026-08-28

### 📦 部署示例

- 新增仓库根目录 `docker-compose.yaml` 示例：内置 GeoIP 无需挂 assets、挂载
  config.yaml/source.yaml/data、healthcheck 探活、日志轮转；附完整用法注释

## [v2.5.2] - 2026-08-28

### 🐛 Docker 构建与运行修复

- Dockerfile / Dockerfile.release 运行镜像**内置 GeoIP 数据库**（Country.mmdb / GeoLite2-ASN.mmdb / version），
  不再依赖外部挂载 assets 目录（缺失时启动 InitGeoIpDB 失败会 os.Exit(1)）
- 修复 .dockerignore 误排除 `log/` Go 源码包导致 `docker build` 失败

## [v2.5.1] - 2026-08-28

### 🐛 日志格式人性化

- slog 由 TextHandler（`time=... level=... msg=...`）改为**自定义 pretty handler**：
  输出 `[2026-08-28 20:18:00] ERROR message`，对齐原 logrus 前缀格式的阅读习惯

## [v2.5.0] - 2026-08-28

### ⚙️ 性能与可靠性

- **SQLite WAL 模式**：读写不互斥，多 goroutine 并发写（健康检查 upsert/冻结/best 保存）
  减少锁冲突；busy_timeout 5s 避免偶发 database is locked；连接池上限 5
- **健康检查并发可配置**：新增 `healthcheck-concurrency`（默认 200，原硬编码 500）
  应用于延迟检测与中转检测
- **冻结节点 TCP 预检**：端口可达才进入完整 URLTest（成本约 1/10），失败直接记失败并剔除，
  降低冻结期检查开销；解锁判定语义不变
- Dockerfile 启用 **HEALTHCHECK**（/health 探活）

### 🔧 日志切换标准库 slog

- log 包内部由 logrus 迁移到 **log/slog**（TextHandler，时间格式对齐原样）；
  保留 `Infoln/Debugln/Warnln/Errorln/SetLevel` 等 API，全项目调用点不变
- 仍静默第三方库（mihomo）写入 logrus 默认 logger 的日志

### 🧪 测试补强（api 包）

- `TestApplyBasePath`：部署前缀剥离逻辑单测
- `TestPageRoutes`：页面/静态资源/订阅端点可达性
- `TestTaskAuthAdminToken` / `TestTaskPublicNoToken`：任务鉴权（401/200）

## [v2.4.1] - 2026-08-28

### 🐛 启动统计恢复

- 修复：启动时首页"可用节点数量/全部节点"显示 0 的问题
- 从数据库恢复上次节点时同步更新全部统计（All/Usefull + 各类型计数），
  启动窗口期内首页即显示库内节点快照，不再等待首轮抓取完成

## [v2.4.0] - 2026-08-28

### ⚡ 部署子路径自动适配（base_path 不再必填）

- 子路径识别优先级：显式配置 `base_path` > 反代 `X-Forwarded-Prefix` 头 > **自动推断**
- 自动推断：从首个请求路径识别已知路由前缀（如 `/show/clash` → 前缀 `/show/`），
  进程内缓存后持续生效；根路径部署不受影响
- **支持多个前缀同时部署**（如 `/show/` 与 `/proxy/` 都转发到本服务）：缓存前缀集合，
  最长前缀优先匹配，逐请求识别新前缀
- **前缀决策移至前端**：订阅/复制/一键导入链接由前端从 `location` 统一渲染（
  `location.origin + 前缀 + 相对路径`），服务端注入 `X-Forwarded-Prefix`/`X-Forwarded-URI`
  解析值兜底 → **适配 nginx 剥离前缀（proxy_pass 带尾斜杠）的标准部署形态**，
  后端无需感知前缀即可输出正确的订阅链接

## [v2.3.1] - 2026-08-28

### 🐛 部署子路径支持

- 新增配置 `base_path`（如 `/show/`）：站点挂载在反向代理子路径下时，
  在 ServeHTTP 层剥离前缀后按根路径路由（兼容反代不剥离前缀的部署形态）
- 全部页面资源/链接改**相对路径**，复制订阅链接自动拼接 `base_path` 前缀
- 修复：部署在子路径时静态文件（CSS/JS）与页面路由 404 的问题

## [v2.3.0] - 2026-08-28

### 🌐 网站功能完善（无 CDN 依赖）

- **CSS/JS 全部本地化**：移除 jsdelivr/bootcdn 外部依赖（页面不再受 CDN 可用性影响）
- **index.js 原生 JS 重写**（去 jQuery）：导航切换 / 复制（clipboard 优先）/ 任务触发
- 首页新增：**优选 IP 统计**（节点/健康/anytls 透传数）、**优选 IP 订阅区块**、**任务区**、v2rayN 导航
- 新增 **`/v2rayn` 导出页**（v2rayN 订阅说明）与 **`/best` 优选 IP 说明页**（7 端点 + 参数 + 探测语义）
- 修正 anytls 订阅链接（指向 best 的 xxxAnytls）、更新仓库链接、lang=zh-CN、移除禁缩放与旧推广位

### 🔐 任务接口鉴权

- 新增配置 `admin_token`：配置后 `/task/*`（crawl/speedtest/updateBestNode/updateGeoIP）
  需携带 `?token=xxx`，否则 401；**留空则保持公开**（向后兼容）
- 前端任务按钮自动携带 Token（localStorage 记忆，可清除）

## [v2.2.0] - 2026-08-28

### 🧊 失效节点冻结机制（解决失效节点被源站持续返回导致永远留库）

- **背景**：原有 7 天清理只按"是否被抓取"判断（`useable`/`updated_at`），失效节点只要每轮
  被源站返回并通过一次健康检查就会持续刷新，永远不会被清理
- 新增健康检查 **streak 状态机**（`pkg/healthcheck`）：连续失败轮数 / 连续通过轮数
- 新增 `proxy_blocklist` 冻结表（持久化，重启不丢）：
  - 连续失败 ≥ `freeze-failures`（默认 3）→ 冻结，**冻结期内即使被采集回来也不入库**
  - 冻结中连续通过 ≥ `unlock-passes`（默认 3）→ 解锁恢复（偶尔通过一次无法复活）
  - 冻结超过 `freeze-window`（默认 30 天）→ 强制解锁（防止节点恢复后永久封禁）
- 冻结节点仍参与健康检查（用于解锁判定），但不参与命名/中转/OpenAI 检测与入库

### 💾 best 节点持久化

- 新增 `best_node` 表：best 节点（含探测标记 AnyTLS/Healthy）落库，全量覆盖式存储（表不增长）
- 启动时 `RestoreBestNodes` 从数据库恢复缓存 → 重启后 `/best*` 订阅**秒级可用**，
  无需等待采集 + 探测完成（探测标记一并恢复）
- 保存时机：采集排序后保存一次，异步探测完成后再次覆盖保存（带标记）

## [v2.1.0] - 2026-08-27

### ⚡ 优选 IP 健康检查（bestip_probe）

- 新增 `bestip_probe` 配置段：用 **vless 协议**（vless+tls+ws）对候选优选 IP 入口做
  完整握手 + 数据往返（URLTest 204）健康检查，标记 `BestNode.Healthy`
- 启用后 **vless / vmess / trojan 格式**（`clashVless`/`surgeTrojan` 等）仅导出
  健康检查通过的节点；未启用时行为不变（向后兼容）
- 探测与 `sni_probe`(anytls) **合并为单个异步 goroutine** 顺序执行、一次写缓存：
  - 先健康检查；全部失败 ⇒ 入口整体不可用，**短路跳过 anytls 探测**
  - 两个标记（Healthy / AnyTLS）独立，互不覆盖
- 凭据复用 `proxy_info[country]["vless"]`（host/uuid/path），构造参数与
  `clashVless` 导出完全一致（探测通过即订阅可用）

### 🧹 清理

- 移除未使用的 Cloudflare API 死配置（`cf_email` / `cf_key` 字段与环境变量覆盖逻辑，
  go.mod 无 cloudflare-go 依赖）

## [v2.0.0] - 2026-08-27

### 🔧 代码现代化（Modern Go 规范，Go 1.27）

- `interface{}` 全部替换为 `any`（45 处 / 17 文件）
- `strings.Cut` / `strings.CutLast` 替代 `Index/LastIndex + 手动切片`（注释剥离、CF 反混淆）
- `slices.SortStableFunc` + `cmp.Compare` 替代 `sort.SliceStable`（优选 IP 多级排序、测速排序）
- `atomic.Pointer[ConfigOptions]` 替代 `atomic.Value + 类型断言`（配置热加载）
- 计数循环 `for i := 0; i < n; i++` → `for i := range n`
- 移除 Go 1.22+ 后冗余的循环变量拷贝（workerpool 闭包）
- **Getter 接口去除 wg 参数**：`Get2ChanWG(pc, wg)` 合并为 `Get2Chan(pc)`，
  调用点改用 `wg.Go`（Go 1.23+）
- workerpool 场景用 `StopWait()` 替代冗余 WaitGroup（CrawlBestNode / anytls 探测），
  净删 90 行并发样板代码
- 无参 `fmt.Errorf` → `errors.New`，修正 `vaild`/`invaild` 拼写

### 🧪 测试

- 新增 `cfdecode`（ScriptReplace）、`SortProxiesBySpeed`、`sortBestNodes` 单元测试（16 用例），
  并在旧实现上验证行为等价
- 优选 IP 多级排序提取为可测函数 `sortBestNodes`
- 新增 `scripts/verify-go.sh`：Docker 内一键 gofmt / vet / build / test 验证

### 📝 文档与发布

- README 重写：anytls 协议、优选 IP 使用、完整 API 文档、默认端口 12580
- 移除 fly.io 部署方式与过时的 `fly.toml`
- 删除过期 `README_NEW.md`，修正 release 说明（Docker 镜像发布，无预编译二进制）

## [v1.1.61] - 2026-08-17

### ⚡ 健康检查测试地址优化（国内可达 204 端点优先，支持配置覆盖）

- **默认测试地址列表重排**（原列表缺陷）：
  - 剔除返回 200 的端点（msftconnecttest / captive.apple / msftncsi / apple.com/test），
    它们在 204 状态码判定下永远失败，白白消耗请求
  - **不含 gstatic**（部署 VPS 常因访问过度被拒/限流）
  - 国内可达的 204 端点排前面：`https://cp.cloudflare.com/generate_204`、
    `https://connect.rom.miui.com/generate_204`，海外端点（google/bing 等）作后备
- **新增配置项 `healthcheck_test_urls`**（可选 []string）：部署环境可覆盖
  默认列表；`CrawlGo` 启动时注入（热更新配置后自动生效）
- 新增 `SetTestURLs`（RWMutex 保护 + 返回副本）与 `TestDefaultTestURLs` /
  `TestSetTestURLs` 测试
- 验证：容器内 cp.cloudflare / 小米 miui 的 generate_204 均可达

## [v1.1.60] - 2026-08-17

### 🚀 探测配置改名 anytls_probe → sni_probe（语义对齐 SNI proxy）

- 配置段 `anytls_probe` → **`sni_probe`**（`SniProbe` / `SniProbeConfig`），
  语义为"best 节点 SNI proxy（TCP 透传入口）可用性探测"；日志统一为 `sni probe`
- **保留真实 anytls 握手验证，不改为普通 HTTPS 探测**。容器内真网络实验证明：
  - `443` 端口对未接入 CF 的域名也会透传握手（104.16.0.1:443 SNI=microsoft.com
    返回真实证书）→ 普通 TLS/HTTPS 探测会把 HTTP 反代误判为可用
  - 探测结果对测试域名高度敏感（microsoft.com 在 2053 失败、example.org 成功）
  - 真实 anytls 握手 + 数据往返（URLTest）才是与用途语义完全匹配的可靠判定
- 配置示例与注释同步更新（config.yaml / bin/*.yaml）
- 兼容性：旧 `anytls_probe` 配置不再识别（等同未配置 → xxxAnytls 导出空），
  升级需改名为 `sni_probe`

## [v1.1.59] - 2026-08-17

### 📝 anytls 探测注释对齐 SNI proxy 术语

- 明确候选 ip:port 的本质是 **SNI proxy（SNI 转发入口）**：只读 TLS ClientHello
  的 SNI 字段、把原始 TCP 流路由到对应域名源站，不解析应用层协议
- `sni` 参数（= proxy_info anytls host）即 SNI proxy 的路由键，必须指向真实
  可用的 anytls 源站域名；443 为 HTTP 反代无法透传，2053/2083/2087/2096/8443
  等端口为纯 SNI 路由 TCP 隧道
- 纯注释/术语更新，逻辑无变化

## [v1.1.58] - 2026-08-17

### 🚀 anytls 探测升级为数据往返验证（URLTest 204）

- **探测判定从"协议握手成功"升级为"数据端到端可转发"**：
  原 `DialContext` 只验证 TCP+TLS+anytls 握手；现改用 mihomo `URLTest`——
  经隧道发起真实 HTTP 请求到 `https://cp.cloudflare.com/generate_204`，
  收到 **204** 才标记可用（握手成功但数据不转发的节点不再误标）
- **测试地址可配置**：`anytls_probe.test_url`（可选，默认
  `https://cp.cloudflare.com/generate_204`），适配不同部署网络
- 探测全部失败时 Warn 提示检查源站（anytls host）可达性与 test_url
- 验证：mihomo URLTest 调用链独立验证通过（direct 出站 + cp.cloudflare.com/204 →
  713ms 无错）；当前示例配置源站 1.top 被 fake-ip 劫持、无真实 anytls 服务，
  端到端 0/389 属正确判定（部署真实源站后生效）

## [v1.1.57] - 2026-08-17

### 🚀 best 节点 anytls 可转发性探测（xxxAnytls 仅导出可透传节点）

- **背景**：anytls 是 TLS over TCP 自定义协议，CF 的 443 anycast 只做 HTTP 层转发，
  仅部分 ip:port 能透传原始 TCP；此前 `xxxAnytls` 导出全部 best 节点，多数不可用
- **探测机制**：`CrawlBestNode()` 收集完成后异步探测每个 ip:port ——
  构造临时 anytls 出站（server=候选 ip, port=候选 port, sni=源站域名），复用 mihomo
  完整握手（TCP+TLS+anytls 协议鉴权），`DialContext` 成功即标记 `BestNode.AnyTLS=true`
- **导出过滤**：`/bestProxyIp/*Anytls`（含 /bestIpKr）仅输出标记节点；
  非 anytls 格式不受影响
- **配置项 `anytls_probe`**（不配置此段时 anytls 导出为空；enable 缺省 true；
  country 缺省自动取 proxy_info 中第一个配置了 anytls 的国家）：
  `enable` / `concurrency`(默认20) / `timeout`(默认5s) / `country`
- **降级**：源站不可用 → 全部探测失败 → anytls 导出空（符合实际）；
  探测异常不阻塞爬取（异步 + 完成后原子替换缓存）
- 新增测试：`anytls_probe` 配置默认值（nil/缺省/显式 false/自定义）、探测国家自动选择、
  导出过滤（启用/关闭/无配置）
- 端到端验证：389 个 best 节点探测标记 **311 个可透传**，`surgeAnytls` 导出恰为 311 行，
  `surgeTrojan` 保持 389 全量

## [v1.1.56] - 2026-08-17

### 🐛 /best* anytls 配置缺失报错提示优化 + /surge/proxies?type=anytls 验证

- **`checkFormat` 错误信息可操作化**：`not found vaild anytls node for country [X],
  add 'anytls: {host, password}' to proxy_info in config.yaml`（vmess/trojan/vless
  同步附带国家名 `[X]`），部署环境未配置 anytls 时不再是一句模糊报错
- **`/surge/proxies?type=anytls` 全链路验证**：新增 `TestSurgeAnyTLSProvide`，
  覆盖 type 精确匹配过滤 → `checkSurgeSupport` 放行 → `ToSurge` 输出；
  无 type 过滤时 anytls 与其他协议同池输出
- 若代理池中无 anytls 节点，`/surge/proxies?type=anytls` 返回空属正常
  （池内无此类节点，需订阅源含 anytls 链接）

## [v1.1.55] - 2026-08-17

### 🚀 anytls 全客户端支持（含 surge，新增 5 个 /best* 格式）

- **Surge 支持 AnyTLS**（iOS 5.17.0+ / Mac 6.4.3+，AnyTLS v2）：
  实现 `AnyTLS.ToSurge()`（`name = anytls, server, port, password=..., sni=...`），
  `checkSurgeSupport` 放行 anytls，`/surge/proxies?type=anytls` 恢复输出
- **`/best*` 新增 5 个 anytls 格式**：`surgeAnytls` / `clashAnytls` / `loonAnytls` /
  `quanxAnytls` / `v2raynAnytls`（含 `/bestCfProxySub` 的 7 参版生成器）
  - surge：`= anytls, ip, port, password=..., sni=...`
  - clash：`type:anytls` + `sni/alpn/skip-cert-verify`（mihomo 原生）
  - loon：`= anytls, ..., "password", over-tls=true, tls-name=...`（Loon 3.3+）
  - quanx：`anytls=ip:port, password=..., over-tls=true, udp-relay=true, tls-host=...`
  - v2rayn：标准 `anytls://password@ip:port?sni=...&alpn=...#name` 链接
- **`checkFormat` 支持 Anytls 类型**：`Format` 新增 `Anytls` 字段；
  未配置 anytls 节点的国家返回 500（与 vmess/trojan 行为一致）
- **`config.yaml` proxy_info 新增 `anytls` 段**（JP/KR）：必填 `host`(sni) + `password`，
  可选 `alpn`(逗号分隔) / `skip_cert_verify`
- 新增测试：`ToSurge` 输出/回退、`checkSurgeSupport` 放行、`generatorKey` 映射、
  5 端生成器输出断言、`checkFormat` 配置校验（12 项）
- 端到端验证：容器内 5 个 `/bestProxyIp/*Anytls` 接口输出全部正确

## [v1.1.54] - 2026-08-17

### 🚀 /bestCfProxySub 支持明文域名列表（bestcf.pages.dev 格式）

- **sub 参数新增支持明文 `host:port#注释` 列表**，如
  `https://bestcf.pages.dev/domain/Domain-Asia.txt`（每行如
  `polestar.com:443#亚洲域名 | 中国极星汽车`，84 行亚洲域名）
- 格式自动识别：base64 解码成功走原 URL 订阅逻辑；失败回退明文行解析，
  两者共用同一去重/生成流水线
- `parsePlainSubLine` 解析规则：host 可为域名或 IP、端口缺失默认 443、
  `#` 后注释作为节点名、端口白名单（443/2053/2083/2087/2096/8443）校验、
  兼容 IPv6 方括号与 CRLF
- 新增 15 个 `TestParsePlainSubLine` 用例
- 端到端验证：容器内实测明文源（surgeTrojan/clashVmess 输出域名节点）与
  base64 旧格式（vless 订阅）均正常

## [v1.1.53] - 2026-08-17

### ⬆️ 运行阶段镜像升级 alpine 3.21 → 3.24

- `Dockerfile` / `Dockerfile.release` 运行阶段：`FROM alpine:3.21` → `alpine:3.24`
  （当前最新稳定版 3.24.1）
- 背景：v1.1.52 升级 Go 1.27 后，构建阶段 `golang:1.27.0-alpine` 底层已是 alpine
  3.24.1，运行阶段 3.21（2024-12 发布）临近 EOL，ca-certificates/tzdata 等
  运行时包同步到新版仓库
- 验证：alpine:3.24 全流程本地构建 + 容器冒烟（/health、首页、/bestProxyIp
  输出含 ipdb 纯 IP 源节点）、容器内确认 alpine 3.24.1

## [v1.1.52] - 2026-08-17

### ⬆️ Go 工具链升级 1.25 → 1.27

- `go.mod`：`go 1.25.0` → `go 1.27.0`
- `Dockerfile`：构建镜像 `golang:1.26.0-alpine` → `golang:1.27.0-alpine`
- CI（`.github/workflows/docker-release.yml`）：`actions/setup-go` 版本 `1.26` → `1.27`
- 验证：Go 1.27.0 下 `go mod tidy` 无依赖变更（go.sum 不变）、`go vet ./...` +
  `go test ./...` 全包通过

## [v1.1.51] - 2026-08-17

### 🚀 sub_ip_list_url 支持纯 IP 行（无端口自动补 443）

- **ipdb 等纯 IP 订阅源支持**：`https://ipdb.api.030101.xyz/?type=bestproxy&country=true`
  返回每行 `IP`（或 `IP#`，无端口），原解析要求 `host:port` 导致整行全部丢弃
- 增强 `parseSubIpSubLine`：无端口行默认补 `443`（在白名单内）；
  有端口行仍严格校验白名单（443/2053/2083/2087/2096/8443）；host 必须为合法 IP
- 兼容裸 IPv6（自动补方括号）；域名/多余冒号等非法行继续丢弃
- 配置示例 `sub_ip_list_url` 追加 ipdb 源
- 新增测试用例：纯 IP 行（带#/无#/带国家注释）、裸 IPv6、多余冒号非法行

## [v1.1.50] - 2026-08-17

### 🚀 新增 sub_ip_list_url 配置项：明文 best IP 订阅源

- **新增配置项 `sub_ip_list_url`**（字符串数组）：明文 `ip:port` 列表订阅源，
  每行格式 `ip:port#国家/地区`（`#` 后为注释，可省略），如
  `104.17.212.191:443#US 🇺🇸`；头部 `# 295 bestips updated at ...` 注释行自动跳过
- **`CrawlBestNode()` 新增第 5 个数据源**：并发拉取 `sub_ip_list_url` 列表，
  解析出 IP 与端口后与其他源（sub_ip_url / cf_best_ip / vps789 Top20 / Provider）
  统一去重 → DNS 解析 → CDN 检测 → GeoIP 国家识别，供 `/bestProxyIp` 等接口输出
- **端口白名单**：仅接受 `443 / 2053 / 2083 / 2087 / 2096 / 8443`，
  其余端口行静默丢弃；host 必须是合法 IP（域名/非法格式行丢弃），兼容 IPv6 方括号
- **失败重试**：与订阅源一致，最多重试 3 次（指数退避 2s/4s/6s）
- 新增单元测试：`parseSubIpSubLine` 解析（标准行/注释/CRLF/白名单/非法端口/IPv6）、
  端口白名单完整性、`config.Parse` 对 `sub_ip_list_url` 的 yaml 解析

## [v1.1.49] - 2026-08-17

### 🎛 type 参数支持多类型 + PORT 环境变量修复

- **`type` 参数支持逗号分隔多类型**：`type=trojan,reality,anytls`
  - 协议类型精确匹配（ss/ssr/vmess/trojan/vless/anytls）
  - **`reality` / `tls` 作为特性匹配**（`type=reality` 匹配 Reality 节点，`type=tls` 匹配 TLS 节点）
  - 适用于 /clash|/surge|/loon|/quanx|/v2rayn/proxies 全部接口
  - 新增 `TestMultiTypeFilter` 覆盖多类型/特性/空类型用例
- **修复 PORT 环境变量覆盖配置端口**：原 `os.Getenv("PORT")` 无条件生效，
  普通部署环境存在 PORT 变量时会被顶到随机端口；改为仅 heroku（DYNO）环境生效

## [v1.1.48] - 2026-08-16

### 🚀 订阅接口支持 tls / reality 过滤 + 启动提速

- **`/ss|/ssr|/vmess|/sip002|/trojan|/vless/sub` 支持 `tls` / `reality` 查询参数**
  （与 /proxies 接口一致，如 `/vless/sub?reality=true` 只返回 Reality 节点）
- **修复启动阻塞**：`cdn.GlobalManager.Update()` 原为同步调用且在 `api.Run()` 之前，
  部分环境访问 Google 受限时（15s 超时+重试≈30s）web 服务迟迟不监听端口；
  改为后台 goroutine 异步加载，启动立即就绪
- 附注：此前冒烟测试观察到的 503 是测试机 curl 走了本机代理（http_proxy）所致，
  与代码无关

## [v1.1.47] - 2026-08-16

### 🧹 CDN 源失败降级为 Warn

- `www.gstatic.com` 等 CDN 源在部分部署环境被访问限制，失败是**预期行为**
  （其它源成功时有降级，总计数正常）
- 5 个外部 CDN 源抓取失败从 `Errorln` 降为 `Warnln`，避免部署日志 ERROR 噪音

## [v1.1.46] - 2026-08-16

### 🐛 CDN IP 段抓取超时与重试

- 部署环境访问 Google(`www.gstatic.com`)TLS 握手超时(网络受限)，
  原实现裸 `http.Get` 无超时无重试，失败直接 Error
- 修复：`pkg/cdn` 统一带超时的 HTTP 客户端(15s) + 一次重试(2s 退避)，
  5 个 CDN 源(fetchTextCIDRs/AWS/Google/Fastly/Gcore)全部接入
- Google 失败仍记 Error(环境限制)，但不会无限挂起；其它源成功时有降级

## [v1.1.45] - 2026-08-16

### ⚡ 测速结果持久化

- 原实现测速结果只存内存（`ProxyStats`），重启后速度标签丢失，需等下次测速（12h）恢复
- **DB `Proxy` 表新增 `Speed` 字段**（AutoMigrate 自动加列，旧库兼容）：
  - `SaveProxyList` 附带最近一次已知速度
  - 新增 `SaveProxiesSpeed`：测速完成后按 identifier upsert 速度
  - `GetAllProxies` 加载时通过 `healthcheck.InitSpeed` 恢复速度，
    启动后速度标签/速度过滤立即可用（无需等测速）
- 新增 `TestSpeedPersistence`：保存→清空统计→重载恢复全链路验证

## [v1.1.44] - 2026-08-16

### 🚀 新增 anytls 协议支持

- **新增 `pkg/proxy/anytls.go`**：AnyTLS 类型 + 链接解析/生成
  （`anytls://password@host:port?sni=...&alpn=...#name`）+ Clash/Loon/QuanX 输出
  （Surge 不支持 anytls，输出为空）
- **接入**：ParseProxyFromLink / ParseProxyFromClashProxy / ToClashMap（mihomo 原生支持，
  已验证解析）/ GrepLinksFromString / checkClashSupport / checkQuanXSupport / checkLoonSupport
- **IsTLS**：anytls 视为 TLS（`tls=true` 过滤包含）
- **统计与页面**：TypeCounts / 缓存变量 / 首页计数与订阅入口 / clash.html type 筛选值
- 新增 5 个单元测试（往返、解析、三端输出、mihomo 解析、链接抓取）

## [v1.1.43] - 2026-08-16

### 🧪 补充 tls/reality 过滤默认行为断言

- 无参数时返回全部节点（默认不区分），`TestTLSRealityFilter` 增加断言

## [v1.1.42] - 2026-08-16

### 🚀 新增 tls / reality 筛选参数

- `/clash|/loon|/quanx|/v2rayn/proxies` 支持 `tls` 与 `reality` 查询参数：
  - `tls=true|1`：仅 TLS 节点（trojan / vmess+tls / vless+tls）
  - `tls=false|0`：仅非 TLS 节点
  - `reality=true|1`：仅 Reality 节点（vless+reality）
  - `reality=false|0`：仅非 Reality 节点
  - 可与现有 type/c/speed 等参数组合
- 新增 `proxy.IsTLS` / `proxy.IsReality` 判断函数（trojan 视为 TLS；reality 仅 vless 有）
- 无筛选判断同步更新，带 tls/reality 参数不命中缓存
- `/clash` 页面筛选参数表格新增两行说明
- 新增 `TestTLSRealityFilter` 组合过滤测试

## [v1.1.41] - 2026-08-16

### 📝 修复 CHANGELOG.md 版本顺序混乱

- v1.1.36 用 `cat >>` 追加导致版本 section 被割成两段
  （开头 35→17，末尾 40→36）；按版本号降序重排为 40→17 连续正序，
  24 个版本内容无缺失无变化

## [v1.1.40] - 2026-08-16

### 🐛 按官方示例最终校准 Loon / QuanX 的 vless 格式

- **Loon**（官方 example.conf）：`tls-name=` 替代 `sni=`（官方 vless 示例用 tls-name），
  其余已匹配（VLESS 大写 / transport= / over-tls= / skip-cert-verify）
- **QuanX**（官方示例 quanx.txt）：
  - `vless=host:port` 去除多余空格（官方无空格）
  - 修复 ws+tls 节点 `obfs` 重复（官方：ws→obfs=wss、tcp+tls→obfs=over-tls，仅设一次）
  - reality 参数独立于传输输出（官方 ws 示例也带 reality-base64-pubkey）
  - 普通 tls 默认 tls-verification，skip-cert-verify 时输出 tls-verification=false
- 测试断言同步更新；修正测试缓存导致的误判（-count=1 强制重跑）


## [v1.1.39] - 2026-08-16

### 🐛 按客户端官方格式修正 Loon / QuanX 的 Reality 输出

- **Loon**（`/loon/proxies`）：补全官方格式
  `name = VLESS, server, port, "uuid", transport=tcp, flow=xtls-rprx-vision,
  public-key="...", short-id=..., over-tls=true, sni=..., udp=true`
  （原实现缺 public-key / short-id / flow / sni / udp，且用 tls-name 而非 sni）
- **QuanX**（`/quanx/proxies`）：补全官方格式
  `obfs=over-tls, obfs-host=..., reality-base64-pubkey=...,
  reality-hex-shortid=..., vless-flow=xtls-rprx-vision`
  （原实现完全没有输出 reality 参数）
- **Clash / v2rayN(Link)**：v1.1.38 已与官方格式一致，无需改动
- 新增 `TestVlessRealityOutput`：断言 Loon/QuanX 的 reality 参数齐全，
  非 reality 节点不输出 reality 参数


## [v1.1.38] - 2026-08-16

### 🚀 补全 vless Reality 与 grpc 传输支持

- **Reality**：解析 `security=reality` 的 `pbk`(public-key) / `sid`(short-id) /
  `spiderX` 参数；`Link()` 生成时 security 标记为 reality 并带回参数；
  Clash 输出 / mihomo 映射输出 `reality-opts:{public-key, short-id}`
- **grpc 传输**：解析 `type=grpc` 的 `serviceName`(兼容 `service_name`)；
  Clash 输出 / mihomo 映射输出 `grpc-opts:{grpc-service-name}`；
  Loon / QuanX 输出 grpc-service-name
- 新增测试：reality 解析/生成往返、grpc 解析/生成往返、
  mihomo 解析 4 种变体（ws+tls / tcp / reality+flow / grpc+tls，reality 公钥运行时生成保证合法）


## [v1.1.37] - 2026-08-16

### 🐛 Surge 不支持 vless，撤销 surge 输出

- Surge 客户端官方仅支持 ss/ssr/vmess/trojan/http/socks5 等，**不支持 vless 协议**；
  v1.1.35 误在 `checkSurgeSupport` 放行 vless，导致 `/surge/proxies` 输出 Surge
  无法使用的 vless 节点
- 修复：移除 `checkSurgeSupport` 的 vless 分支（恢复为不支持），
  `/surge/proxies?type=vless` 返回空为**正确行为**
- 更新回归测试：surge 断言为拒绝 vless，clash/loon/quanx 仍放行


## [v1.1.36] - 2026-08-16

### 🐛 紧急修复：v1.1.33 破坏全部 HTML 页面

- v1.1.33 用正则脚本批量改导航栏时，`src = group(1) + nav_new + group(3)`
  先把文件覆盖成导航栏碎片，又执行了一次正则替换，导致 **6 个页面全部被压成
  9 行坏文件**（index/clash/surge/shadowrocket 在 v1.1.33 被提交为坏版本；
  loon/quanx 因 .gitignore 的 `config/*` 规则从未入库）
- 修复：从 v1.1.32 恢复 4 个页面（保留 vless 相关修改），用**精确字符串替换**
  重新加入 Loon/QuanX 导航，重新生成 loon.html / quanx.html
- 修复 .gitignore：`config/*` 宽泛规则改为明确忽略 yaml/crt/key/ini，
  放行 `config/assets/html/*.html` 与 `static/index.js`（页面模板必须入库），
  补回 `proxypool*`、`data`、mmdb 忽略
- 验证：6 个页面全部 200 渲染、结构完整（doctype/html/nav/footer 齐全）、
  导航含 Loon/QuanX

## [v1.1.35] - 2026-08-16

### 🐛 修复 /loon|/surge/proxies 的 vless 输出为空

- v1.1.31 新增 vless 时漏改 `checkLoonSupport` / `checkSurgeSupport`：
  两个校验函数没有 vless 分支，导致 vless 节点被 Loon / Surge 输出排除，
  `/loon/proxies?type=vless`、`/surge/proxies?type=vless` 返回空
- 修复：两个校验函数补充 `case *proxy.Vless: return true`
  （vless 类型自身已有 ToLoon / ToSurge 实现）
- 新增回归测试：`TestVlessSupport` 覆盖 clash/loon/surge/quanx 四个校验函数


## [v1.1.34] - 2026-08-16

### 🐛 静默 mihomo 内部日志噪音

- 健康检查/测速时 mihomo 内部（如 vless vision 握手失败）会把 ERROR 级日志
  （`XTLS Vision server responded unknown UUID`、`vision: not a valid...`）
  刷到应用日志——mihomo 与应用此前共用 logrus **默认 logger**
- 修复：应用改用**独立 logrus 实例**，默认 logger 输出丢弃（`io.Discard`）；
  应用日志不受影响，第三方库内部噪音全部静默
- 这些错误本身是无效/过期节点的正常检测信号（节点会被健康检查过滤），
  但大量节点检测时噪音可观，隔离后日志干净
- 新增回归测试：默认 logger 输出已丢弃、应用日志正常


## [v1.1.33] - 2026-08-16

### 🚀 新增 /loon /quanx 页面与 QuanX 输出

- **新增页面**：`/loon`、`/quanx`（与 /clash /surge 同风格，含节点列表订阅入口）
- **新增 `ToQuanX()`**：ss / ssr / vmess / trojan / vless 五种类型的 QuanX 格式输出
  （`vmess = host:port, method=..., password=..., obfs=wss, ...`）
- **新增 QuanX provider**：`/quanx/proxies` 接口（支持现有全部筛选参数）
- 所有页面导航栏增加 Loon / QuanX 入口（6 个页面同步）
- 新增 ToQuanX 输出单元测试


## [v1.1.32] - 2026-08-16

### 🎨 页面与文档同步 vless

- `/clash` 页面：meta 描述、type 筛选值表格增加 vless
- `/surge`、`/shadowrocket` 页面：meta 描述增加 vless
- `index.html`：订阅表格改用专用 `/vless/sub` 入口（原 clash 筛选链接）
- README 协议描述增加 vless
- **新增 `/vless/sub` 订阅接口**：`VlessSub` provider，base64 编码的 vless:// 链接列表
  （与 vmess/sub 同模式）


## [v1.1.31] - 2026-08-16

### 🚀 新增 vless 协议支持

- **新增 `pkg/proxy/vless.go`**：Vless 类型 + 链接解析/生成
  （`vless://uuid@host:port?...`）、三端输出（Clash/Surge/Loon）、去重标识
- **解析入口接入**：`ParseProxyFromLink` / `ParseProxyFromClashProxy`
  （含 clash 配置 `ws-opts` 嵌套结构兼容）
- **抓取接入**：`GrepLinksFromString` 支持 vless 链接（正则要求 `uuid@host:port` 结构，
  避免把普通文本误抓为链接）
- **健康检查/测速接入**：`ToClashMap` 输出 mihomo 所需键名（`ws-opts`/`servername`/
  `client-fingerprint`/`flow` 等），已验证 mihomo 可解析 ws+tls / tcp / reality+flow 三种变体
- **Clash 支持校验**：`checkClashSupport` 放行 vless（mihomo 原生支持，无加密白名单限制）
- **统计与首页**：`TypeCounts`/缓存变量/首页模板增加 vless 计数与订阅入口
- 新增 5 个单元测试（往返解析、ws 参数、三端输出、链接抓取、clash 配置兼容、mihomo 解析）


## [v1.1.30] - 2026-08-16

### 🐛 修复健康检查 done 通道二次关闭 panic

- `CleanBadProxiesWithWorkpool`：`done` 通道既被 goroutine `close(done)`
  （StopWait 完成后）又被 `defer close(done)` 关闭，健康检查正常完成、
  循环经 done 分支返回时触发 `close of closed channel` panic
- 之前未暴露的原因：v1.1.26 修复前的 nil 指针 bug 先 panic，掩盖了此问题；
  nil 修复后健康检查能完整跑完（9426/9426），二次关闭立即暴露
- 修复：移除 `defer close(done)`，done 仅由 goroutine 单一关闭
- 新增空列表回归测试：空列表不提交任务，StopWait 立即返回并 close(done)，
  旧实现必然二次关闭 panic（无需网络即可复现）


## [v1.1.29] - 2026-08-16

### 🐛 修复测速任务未重读配置（回归）

- v1.1.23 把任务收敛到 `internal/app` 时，`SpeedTestTask` / `ActiveSpeedTestTask`
  丢失了原先 cron 中的 `config.Parse("")` 调用
- 后果：动态修改 `config.yaml`（如开启 `speedtest: true`）后，测速定时器触发时
  读到的仍是启动时的旧配置，开关不生效
- 修复：两个测速任务每次触发先重读配置（mtime 缓存，未变化零开销）

> 注：定时任务的**间隔**在启动时固化（`cron.Cron()` 读取一次），
> 修改 `crawl-interval` / `speedtest-interval` 等需重启进程；
> 开关类配置（如 `speedtest`）随任务触发热加载，或手动 `GET /task/speedtest` 立即生效


## [v1.1.28] - 2026-08-16

### 🐛 修复 /proxies 接口节点名缺失

- **`GetAllProxies` 恢复数据库保存的 name/country**：原先只取 `link` 字段重新解析，
  启动加载阶段（首轮爬取完成前）`cache.SetProxies("proxies", dbProxies)` 里的节点
  全部是空名称，导致 `/loon|surge|clash/proxies` 输出 ` = Shadowsocks,...`（name 缺失）
- **修复 `ParseProxyFromLink` 的 GeoIP 错误泄漏 bug**：GeoIP 查询失败时错误原样返回，
  导致 `GetAllProxies` 的 `if err == nil` 把解析成功的节点全部丢弃；
  现 GeoIP 仅用于补充国家信息（失败用 `🏁 ZZ` 默认值），不再否决节点
- 新增数据库回归测试：验证加载时恢复 name/country、解析字段正确


## [v1.1.27] - 2026-08-16

### ⚡ 站点缓存优化

- **带 `random=true` 的请求（`/bestProxyIp`、`/bestIpKr` 随机打乱节点）跳过缓存**：
  否则 1 分钟内多次调用会返回完全相同的随机排列，失去随机语义
- **缓存 key 改用 `URL.RequestURI()` 重建**（含 query）：不依赖服务端填充的
  `RequestURI` 字段，httptest 等场景也能正确区分不同请求，避免 key 塌缩
- 测试补充：random=true 绕过缓存、无 random 参数仍被缓存、不同路径 key 独立


## [v1.1.26] - 2026-08-16

### 🐛 修复健康检查 nil 指针 panic

- **修复 `CleanBadProxiesWithWorkpool` 的变量遮蔽 bug（严重）**：v1.1.22 重写时把
  `c <- ps` 移到 if/else 外，但 if 语句的 `:=` 声明了遮蔽外层的新 `ps`，
  两个分支赋值的是内层变量，channel 实际发送的是外层 nil 指针，
  接收端 `ps.Delay` 触发 `invalid memory address or nil pointer dereference`，
  首次爬取健康检查阶段即崩溃
- 修复方式：find-or-create 逻辑提取为 `findOrCreateDelayStat`（锁内查找/创建，
  保证返回非 nil），worker 发送其返回值
- 新增回归测试 `TestFindOrCreateDelayStat`：验证首次创建/二次命中更新均返回非 nil


## [v1.1.25] - 2026-08-16

### 🐛 修复订阅源抓取超时

- `CrawlBestNode` 订阅源请求超时 30s → **60s**：v1.1.24 引入共享 resty 客户端时
  设置 30s 超时（原实现无超时会无限挂起），但部分海外订阅源响应较慢，
  30s 偏紧导致 `context deadline exceeded`；60s 在防挂死与容错间取得平衡
- 重试退避 1s/2s → **2s/4s**：超时后给慢源更多恢复时间，避免立即重试再次超时


## [v1.1.24] - 2026-08-16

### ⚡ internal/app 优化

- **5 份重复的 URL 生成器 15 分支 switch 收敛**：`generatorKey(f)` + 两张生成器表
  （6 参 / 7 参），原先每个 SubNice* 函数各内联一份
- **`SubNiceCfProxyIp` / `SubNiceCfProxyIpTop20` / `SubNiceCfProxyIpProvider`
  收敛为薄封装**：共用 `buildNodeOutput`，每个由 ~110 行缩至 ~25 行
- **公共逻辑提取**：`loadProxyInfo`（5 份 copier.Copy）、`writeOutputHeader`（5 份头部）、
  `finishOutput`（4 份 V2rayn base64）、`trackDuration`（5 份 defer 计时）
- **`CrawlBestNode`**：`resty.New()` 每请求创建 → 包级共享客户端（含 TLS 跳过校验/UA/超时）；
  三处手写补端口 → `normalizeAddr`（顺带修复裸 IPv6 无方括号的隐患）；
  两处手写 host:port 拆分 → `splitHostPort`（net.SplitHostPort 正确支持 IPv6）
- **`CfIpProvider` 去重**：4 个相同匿名 struct + fetchCfIpProvider 中重复的 1 份 → 命名类型 `cfIpItem`
- **单次遍历统计类型数量**：新增 `ProxyList.TypeCounts()`，替代 8 次 `TypeLen` 的 O(8n) 扫描
- **`CrawlGo` 末尾复用 `RefreshProviderCache`**，删除内联的三份 provider 刷新
- 删除死代码 `removeDuplicateElement`、`filterIpCountry`
- `internal/app` 净减约 620 行（1696 → 1159 行的核心文件）


## [v1.1.23] - 2026-08-16

### ⚡ API 路由与配置加载优化

- **`config.Parse` 加缓存**：本地文件按 mtime 判断是否变化（保留热更新），
  http(s) 源按 60s TTL 刷新。原先 `/best*` 接口每次请求都重新读文件/网络 + YAML 解析，
  URL 配置源则每次请求都发 HTTP GET；现未变化时零开销
- **修复站点缓存中间件**：gin-contrib/cache 的 `SiteCache` 只读不写（从不填充缓存，
  等价于空操作）；改为自实现中间件——未命中时包装 ResponseWriter 记录 2xx 响应，
  命中时直接回放，HTML 页面/订阅接口获得真实 1 分钟 HTTP 缓存
- **触发类接口排除缓存**：`/task/*`、`/health`、`/link/`、`/debug/*` 跳过响应缓存，
  确保手动触发任务/健康检查/动态链接不被缓存拦截
- **7 个 `/best*` handler 收敛**：统一 `bestIPHandler`（配置加载+错误处理）、
  `parseBestIPParams`（d 默认 JP）、`isTrue`（"true"/"1"）辅助函数，
  每个 handler 由 ~50 行缩至 3-6 行
- **5 个 `/xx/sub` handler 收敛**为 `subHandler` 一行式注册
- **任务统一收敛 `internal/app`**：`CrawlTask`/`SpeedTestTask`/`ActiveSpeedTestTask`/
  `BestNodeTask`/`GeoIPTask` 供 cron 与 API 共用，消除重复逻辑；
  `RunExclusive` 互斥防并发——任务运行中再次触发直接跳过（日志提示），
  避免 cron 与手动触发并发抓取/测速
- `/task/*` handler 不再各自重复 `runtime.GC()`（收敛到 RunExclusive）
- 新增单元测试：`config.Parse` 缓存命中/失效、站点缓存命中/排除


## [v1.1.22] - 2026-08-16

### 🔧 CI/CD

- Docker 构建改为**仅 tag 推送自动触发**；master/main 分支改为手工触发（`workflow_dispatch`）

### ⚡ 健康检查与测速优化

- **消除 JSON 往返转换**：新增 `proxy.ToClashMap()` 直接由结构体构造 mihomo 配置 map，
  替代健康检查/测速/中转检测中每次的 `String()`→`json.Unmarshal`，
  并新增等价性单元测试（与旧行为 JSON 形态完全一致）
- **延迟检测减少无效请求**：每轮重试最多尝试 4 个测试 URL（原 10 个），
  健康代理仍 2 个成功即返回，失效代理判定时间大幅缩短
- **测速服务器列表缓存**：speedtest 静态服务器 XML（~100KB）缓存 1 小时，
  避免每个代理重复下载；取列表/用户接口超时 5s → 15s，慢代理不再下载不完直接失败
- **用户位置获取失败降级**：原先直接判定测速失败，现退化为使用服务器列表前 3 个，
  不再因 config 接口被墙而整批测速失败
- **合并重复测速函数**：`SpeedTestAllWithWorkpool` 与 `SpeedTestNewWithWorkpool`
  收敛为单一实现（newOnly 参数区分）


## [v1.1.21] - 2026-08-16

### ⚡ GeoIP 数据库处理优化

- **本地持久化**：Country.mmdb 下载后落盘 + 版本号写入 `assets/version`，
  重启直接本地加载（秒开），不再每次启动重复下载 40MB 到内存
- **修复版本追踪 bug**：原实现本地文件模式版本号为空，cron 的 `UpdateGeoIP` 永远跳过更新检查；
  现每次启动读取本地版本，每日 cron 与远程比对，有更新才下载
- **修复并发竞态**：`UpdateGeoIP`/`UpdateGeoIpASNDB` 原实现直接替换 `GeoIpDB.db` 并立即
  Close 旧 reader，与进行中的 `Find`/`GetASN` 冲突；改用 `atomic.Pointer` 原子替换 + 延迟关闭
- **下载超时**：原裸 `http.Client{}` 无超时、失败即 panic；现统一 30s/10s 超时客户端，失败返回错误
- **修复 ASN 库下载源**：`git.io` 短链已被 GitHub 停用且不稳定，改用 GitHub Release 直连地址
- **启动不再强制下载 ASN 库**：本地已有则直接加载，仅缺失时才下载
- **ip-api.com fallback 防限流**：加 TTL 缓存（成功 24h / 失败 5min）+ 串行化查询 + 5s 超时，
  避免代理池大量查询触发 429
- **`Find` 微优化**：纯 IP 入参直接解析跳过 DNS 查询；国家名拼接用字符串连接替代 `fmt.Sprintf`
- `IsCDN` 关键词表大写预编译；删除死代码 `ReInitGeoIpDB`/`GeoIpBinary`/`GeoIpVersion`
- 新增 5 个单元测试（emoji 映射 / 原子写文件 / 无库容错 / 非法输入 / 关键词覆盖）


## [v1.1.20] - 2026-08-16

### 🔒 安全修复

- 升级 `github.com/quic-go/quic-go` v0.59.0 → v0.61.0，修复 CVE-2026-40898（medium）
- 该库由 gin v1.12 的 HTTP/3 支持引入（`quic-go/quic-go/http3`），
  至此 Dependabot 报告的 25 个漏洞全部清除


## [v1.1.19] - 2026-08-16

### 🔧 CI/CD 优化

- **修复镜像构建不触发的严重 bug**：workflow 仅在 `branches: ['main']` 触发，
  但默认分支为 `master`，导致推送到 master 从不构建镜像；现同时监听 `master` + `main`
- 新增 `workflow_dispatch` 手动触发能力
- 新增 `concurrency` 控制：同一 ref 的重复触发自动取消旧任务，避免并发构建浪费
- 新增 `go vet` + `go test` 步骤，在构建前拦截代码质量问题
- 二进制编译改为并行（`make linux-amd64 &` + `make linux-armv8 &`），缩短构建时间
- 新增 `.dockerignore`：排除 `.git`(107MB)、`bin/` 旧产物(277MB)等，
  大幅缩减 Docker 构建上下文上传量
- 新增 `.github/dependabot.yml`：GitHub Actions 每周自动升级并生成可验证的 PR
- `actions/checkout` v4 → v5

### 🧪 测试

- 修复 `pkg/geoIp` 测试在 CI 上 panic 的问题：GeoIP 数据库缺失且联网下载失败时
  优雅 `t.Skip` 跳过，而非 panic 导致整个测试失败


## [v1.1.18] - 2026-08-16

### 🔒 安全修复

修复 Dependabot 报告的影响本分支（`ai`）的 3 个依赖漏洞：

| 依赖 | 变更 | 漏洞 |
| --- | --- | --- |
| golang.org/x/crypto | v0.50.0 → v0.55.0 | 13 个 CVE（7 critical / 2 high / 4 medium），含 CVE-2026-39830/39831/39832/39833/39834/42508/46595 等 |
| golang.org/x/net | v0.53.0 → v0.58.0 | CVE-2026-25680 |
| github.com/antchfx/xpath | v1.3.5 → v1.3.8 | CVE-2026-32287（high） |

说明：
- Dependabot 告警按默认分支（`master`）评估，其中 pgx/v5、go-retryablehttp 相关告警
  仅存在于 `master` 的依赖图，本分支不受影响
- 升级为间接依赖，与 mihomo 等上游要求（x/crypto ≥ v0.33、x/net ≥ v0.35）兼容


## [v1.1.17] - 2026-08-16

### 🚀 新增

- 新增 `progress` 并发安全进度统计助手：批量节点检测时每完成 10% 输出一条日志，
  替代旧实现中每个节点都向 stdout 刷屏的 `fmt.Printf`（并发下输出交错混乱）
- `pkg/healthcheck` 新增线程安全的 `ProxyStats` 访问 API（`FindStat` / `AppendStat` / `IncReqCount`）

### ⬆️ 依赖升级

| 依赖 | 变更 | 说明 |
| --- | --- | --- |
| github.com/gin-gonic/gin | v1.11.0 → v1.12.0 | Web 框架 |
| github.com/metacubex/mihomo | v1.19.20 → v1.19.29 | 核心代理引擎（连带升级大量传递依赖） |
| github.com/sirupsen/logrus | v1.9.4 → v1.10.0 | 日志 |
| gorm.io/gorm | v1.31.1 → v1.31.2 | ORM |
| github.com/arl/statsviz | v0.8.0 → v0.8.1 | 运行时可视化 |
| github.com/gin-contrib/cache | v1.4.1 → v1.4.4 | 页面缓存中间件 |
| github.com/gin-contrib/pprof | v1.5.3 → v1.5.4 | pprof 中间件 |
| github.com/heroku/x | v0.5.3 → v0.6.1 | Heroku 平台集成 |

**替换已停更/归档的依赖：**

- `github.com/ghodss/yaml`（已归档）→ `gopkg.in/yaml.v3`
- `github.com/patrickmn/go-cache`（已归档）→ 内部自研 `cacheStore`（RWMutex 实现，更轻量）
- `github.com/jasonlvhit/gocron`（2018 年停更）→ `github.com/go-co-op/gocron/v2`（活跃维护）
- `github.com/ivpusic/grpool`（停更）→ `gammazero/workerpool`，并删除全部 grpool 死代码
- `golang.org/x/exp/slices` → 标准库 `slices`（Go 1.21+ 已内置）

### 🐛 Bug 修复

- **`ClearOldItems` 清理功能从未生效**：gorm 的 `Delete` 返回 `*gorm.DB` 而非 `error`，
  原判断恒为真，导致超过一周的失效代理从未被清扫；现改为检查 `.Error` 与 `RowsAffected`
- **`SaveProxyList` 事务范围错误**：事务内使用 `DB` 而非 `tx`，操作未真正纳入事务；现已统一使用 `tx`
- **`/link/:id` 错误处理缺少 `return`**：参数非法或越界时仍继续执行，存在 panic 风险
- **测速结果记录逻辑恒真**：`err == nil || speed > 0` 等价于 `err == nil`，
  0 速度（白名单外/h2 节点）会被写入统计并污染排序；改为 `err == nil && speed > 0`
- **`NameAddIndex` 格式化错误**：`%+02v` → `%02d`，节点序号命名现在输出正确补零格式
- **`main.go` 格式串 vet 警告**：`log.Errorln(err.Error())` 将错误消息当作格式串，
  含 `%` 的错误消息会被错误解析；改为显式 `"%s"` 占位
- **`Update("useable", "false")` 字符串写入**：重置可用状态时写入字符串 `"false"`（SQLite 中为真值），
  现改为布尔值 `false`，配合清理逻辑生效

### 🔧 性能优化

- **数据库批量写入**：`SaveProxyList` 由逐条 `Create` + `Update` 循环改为
  `CreateInBatches` + `clause.OnConflict` 批量 upsert，SQL 往返次数大幅减少
- **`ReqCountThan` 复杂度 O(n×m) → O(n+m)**：改用 map 索引替代嵌套循环
- **`SortProxiesBySpeed` 冒泡排序 → `sort.SliceStable`**：O(n²) → O(n log n)，
  并预取速度记录避免比较器内线性查找
- **`pkg/healthcheck/util.go` 去重**：5 个几乎相同的 HTTP 代理请求函数合并为
  单一 `doViaProxy` 核心 + 薄封装；读完整响应体以支持连接复用
- **`rand.Shuffle` 简化**：Go 1.20+ 全局随机源自动播种，删除手动
  `rand.New(rand.NewSource(time.Now().UnixNano()))` 的重复构造
- **API 路由去重**：`/clash|/surge|/loon|/v2rayn/proxies` 四个 handler 中重复的
  参数解析/缓存/筛选逻辑收敛为统一的 `serveProxyList` 助手

### 🧵 并发安全修复

- **`ProxyStats` 全局统计竞态**：HTTP 请求（`provider.preFilter`）与爬取任务
  （延迟/测速/中转/ChatGPT 检测）会并发读写 `ProxyStats`，存在数据竞争；
  引入包级 `statsLock`（RWMutex）与线程安全 API，统一所有访问路径
- **各检测函数局部锁替换**：`delaycheck` / `speedcheck` / `relaycheck` /
  `openaicheck` 中的局部 `sync.Mutex` 统一改为包级 `statsLock`，
  修复 `Find` 在锁外、`UpdatePSSpeed`/`UpdatePSCount` 在锁外等竞态
- **非原子进度计数**：`doneCount++` / `fmt.Printf` 进度输出（数据竞争 + 输出乱码）
  替换为 `atomic.Int64` 驱动的 `progress` 助手

### 🧹 清理

- 删除死代码：grpool 版本的 `SpeedTestAll` / `SpeedTestNew` / `RelayCheck`、
  `CleanBadProxiesWithGrpool` 及大段注释代码
- 废弃 API 替换：`ioutil.ReadAll` / `ReadFile` / `WriteFile` → `io.ReadAll` / `os.ReadFile` / `os.WriteFile`
- 移除已替换依赖在 `go.mod` / `go.sum` 中的条目
- 变更规模：28 个文件，+701 / -1270 行（净减约 570 行）

