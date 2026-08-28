# best IP 健康检查探测设计方案

> 目标：为 best IP 节点增加可用性检查，方案与现有 anytls 探测（`sni_probe`）对称，
> 探测协议使用 **vless**（vless + tls + ws）——vless 仅是探测手段，标记的是优选 IP 入口本身可用。

## 一、背景与语义

现有 anytls 探测验证"候选 ip:port 是否为可用的 SNI proxy（SNI 转发入口）"：
构造临时出站（server=候选 ip, port=候选 port, sni=源站域名）→ mihomo `URLTest`
→ 完整握手 + 数据往返（204）。探测通过标记 `AnyTLS=true`，仅 `xxxAnytls` 格式导出。

**vless 探测完全同构**：vless 同为 TLS 承载协议，SNI proxy 按 ClientHello 的 SNI 路由。
用 vless 做探测 = 验证"该 ip:port 能否端到端转发 vless 流量"。

## 二、配置（新增 `bestip_probe` 段，与 `sni_probe` 平行、独立开关）

```yaml
bestip_probe:
  enable: true        # 段存在且未写 enable = 默认 true；显式 false 关闭；整段缺失 = 未启用
  concurrency: 20     # 并发探测数，默认 20
  timeout: 5          # 单节点超时（秒），默认 5
  country: "JP"       # 使用哪个国家的 vless 凭据；缺省自动选第一个含 vless 段的国家
  # test_url: "https://cp.cloudflare.com/generate_204"   # 可选，覆盖数据往返验证地址
```

凭据来源：`proxy_info[country]["vless"]` = `{host, uuid, path}`（现有配置，无需新增）。

## 三、数据结构

| 文件 | 改动 |
|------|------|
| `internal/cache/cache.go` | `BestNode` 增加 `Healthy bool（优选 IP 健康检查通过）`（与 `AnyTLS` 并列，独立标记） |
| `config/config.go` | `ConfigOptions` 增加 `BestIPProbe *BestIPProbeConfig`；新增 `BestIPProbeConfig`（字段同 `SniProbeConfig`） |

## 四、探测逻辑（新文件 `internal/app/bestip_probe.go`）

1. **`probeBestIPCountry(cfg)`**：配置 `country` 优先（该国家须含 `vless` 凭据）；缺省遍历
   `proxy_info` 选第一个含 vless 段的国家。无可用国家 → 不探测（vless 导出为空 + 告警）。
2. **`probeBestIPNode(ip, port, uuid, sni, wsPath, testURL, timeout)`**：
   - 构造 `proxy.Vless`：`Server=ip, Port=port, UUID=uuid, ServerName=sni(host),
     WSPath=path, Network="ws", TLS=true, UDP=true, Fingerprint="chrome"`
   - **参数与 `genClashVlessUrl` 输出完全一致** → 保证"探测通过 = 该格式订阅可用"
   - `adapter.ParseProxy(proxy.ToClashMap(v))` → `URLTest(ctx, testURL, 期望 204)`
3. **`ProbeAndMarkHealthy(nodes)`**：workerpool 并发（`bestip_probe.concurrency`），逐节点标记
   `Healthy bool`，返回新切片（不修改入参）。
4. **`filterHealthyNodes(nodes)`**：`bestip_probe` 未启用 → 导出空（与 anytls 行为一致）。

## 五、与 anytls 探测的协调（关键）

现状：`CrawlBestNode` 尾部对 anytls 是独立 `go func` 异步探测，完成后
`cache.SetBestNodeList` 替换缓存。**若再加一个独立 goroutine，两个探测并发写缓存会互相
覆盖标记**（后完成的把先完成的标记冲掉）。

**方案：合并为单个异步 goroutine**，顺序执行（先 vless 后 anytls），合并标记后**一次**写缓存：

```go
// CrawlBestNode 尾部
func ProbeAndMarkNodes(base []cache.BestNode) []cache.BestNode {
    marked := base
    // vless 探测：通用入口可用性前置检查
    if cfg := config.Config().BestIPProbe; cfg != nil && cfg.Enabled() {
        marked = ProbeAndMarkHealthy(marked)
        // 短路：vless 全部失败 → 入口不可用，anytls 探测无意义，跳过
        if countVless(marked) == 0 {
            log.Warnln("vless probe: all nodes failed, skip anytls probe")
            return marked
        }
    }
    if cfg := config.Config().SniProbe; cfg != nil && cfg.Enabled() {
        marked = ProbeAndMarkAnyTLS(marked)
    }
    return marked
}
// CrawlBestNode 尾部：go func() { cache.SetBestNodeList("bestNode", ProbeAndMarkNodes(bestNodeList)) }()
```

- **短路语义**：vless 是通用入口可用性检查（TLS 数据能否端到端转发），全部失败 ⇒ 入口整体
  不可用，anytls 探测大概率同样失败，跳过以节省探测时间/资源；此时 `xxxAnytls` 导出为空
- 仅启用其一 → 只跑其一，行为与现在一致；两者都启用 → 顺序执行 + 短路 + 一次写缓存

## 六、导出过滤范围（已确认：优选 IP 通用入口可用性）

**确认方案**：`bestip_probe` 启用时，vless 探测是对**优选 IP 入口本身的可用性验证**
（能端到端转发 TLS 数据 = 可用 SNI proxy 入口），因此 **vless / vmess / trojan 协议格式**
（`f.Vless || f.Vmess || f.Trojan`）全部只导出 `Vless=true` 的节点；未启用时行为不变
（**向后兼容**）。

- `xxxAnytls` 格式仍由 `sni_probe` 的 `AnyTLS` 标记独立过滤（两个探测各自标记、各自生效）
- 两个探测**合并为单个异步 goroutine** 顺序执行，一次写缓存，标记互不覆盖

## 七、测试

参照 `anytls_probe_test.go`（不依赖网络，纯逻辑）：
- `TestProbeVlessCountry`：配置指定 / 自动选择 / 指定国家无 vless 凭据失败
- `TestFilterVlessNodes`：启用→仅标记节点 / 显式关闭→空 / 无配置段→空
- 网络探测（URLTest）不单测，交付前 Docker 冒烟验证（参考 anytls 冒烟）

## 八、文件改动清单

| 文件 | 改动 |
|------|------|
| `internal/cache/cache.go` | `BestNode` + `Healthy bool` |
| `config/config.go` | + `BestIPProbe *BestIPProbeConfig` + 结构体定义 |
| `internal/app/bestip_probe.go` | 新增：国家选择 / 单节点探测 / 批量标记 / 过滤 |
| `internal/app/best_proxy_ip.go` | CrawlBestNode 尾部合并探测 goroutine；`SubNiceProxyIp` 中 `f.Vless` 时过滤 |
| `internal/app/bestip_probe_test.go` | 新增测试 |
| `config/config.yaml` | + `bestip_probe` 配置示例 |
| `README.md` | 配置表 + 说明 |
| `CHANGELOG.md` | v2.1.0 条目 |

## 九、风险与说明

- 探测是异步的（不阻塞爬取），订阅请求可能命中探测完成前的缓存（anytls 现状相同）
- `BestNode.Vless` 标记随缓存持久，探测完成后自动替换
- vless 探测对 443 端口同样可能通过（vless+ws 常见于 443 反代），不做端口预排除，以实际握手结果为准
- 版本：新功能 → **v2.1.0**（changelog + tag）
