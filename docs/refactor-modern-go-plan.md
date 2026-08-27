# Modern Go 规范优化方案

> 分支：`refactor/modern-go`（已基于 master 创建，无额外提交，工作区干净）
> 目标 Go 版本：`go 1.27.0`（go.mod）
> 依据：Modern Go Guidelines CLI（`list --go-version 1.27` 全量输出）

## 一、现状扫描结果

- 仓库共 86 个 `.go` 文件，约 11.7k 行；依赖较多（gin / gorm / mihomo / colly 等）。
- CI 校验标准：`go vet ./...` + `go test ./...`（`.github/workflows/docker-release.yml`）。
- 本机无 Go 工具链，但有 Docker（Dockerfile 使用 `golang:1.27.0-alpine`），可用容器完成编译/测试/vet 验证。

## 二、优化清单（按风险从低到高）

### P0：纯机械替换，零行为变化

| # | 规范 ID | 改动 | 位置 | 数量 |
|---|---------|------|------|------|
| 1 | `any` | `interface{}` → `any` | clashmap.go(10)、best_proxy_ip.go(6)、log/log.go(5)、vmess.go(4)、vless.go(4)、shadowsocks.go(2)、getter/base.go(2)、helpers.go(2)、option.go、proxy/base.go、anytls.go、sssub.go、openaicheck.go、getter/clash.go、config/proxy_info.go 及测试文件(3) | 45 处 |
| 2 | `strings_cut` | `strings.Index(x, sep)+切片` → `strings.Cut` | internal/app/best_proxy_ip.go:839, 869（去 `#` 注释） | 2 处 |
| 3 | `strings_bytes_cut_last` | `strings.LastIndex(x, "}")+切片` → `strings.CutLast` | pkg/tool/cfdecode.go:109, 143 | 2 处 |
| 4 | `range_over_int` | `for i := 0; i < n; i++` → `for i := range n` | best_proxy_ip.go:904（`< 4`）、:59/:164（重试 `retries < 3`）、speedcheck.go:188（`< 2`） | 4 处 |

### P1：小范围重构，行为等价，需测试回归

| # | 规范 ID | 改动 | 位置 |
|---|---------|------|------|
| 5 | `atomic_types` | `gCfg atomic.Value` → `atomic.Pointer[ConfigOptions]`，删除 `Load()` 后的类型断言 `v.(*ConfigOptions)` | config/config.go:74, 94, 208 |
| 6 | `loopvar_capture` | 删除冗余循环变量拷贝 `i, node := i, node`（Go 1.22+ 每次迭代独立变量） | internal/app/anytls_probe.go:120 |
| 7 | `loopvar_capture` | 同上删除 `pDB := proxyDB` | internal/database/proxy.go:105 |
| 8 | `slices_sort_func` | `sort.SliceStable` → `slices.SortStableFunc`（闭包改为 `func(a, b T) int`，可用 `cmp.Compare` 简化多键比较） | internal/app/best_proxy_ip.go:320（国家/IP/端口多级排序，IP 数值比较需保留）、pkg/healthcheck/statistic.go:172（测速记录优先排序） |

### P2：并发结构优化（最大收益点，需重点回归）

| # | 规范 ID | 改动 | 位置 |
|---|---------|------|------|
| 9 | `sync_waitgroup_go` | `wg.Add(1); go fn()` → `wg.Go(fn)` | internal/app/task.go:30-34；internal/app/best_proxy_ip.go:48-53, 103, 120, 136, 158 各 `wg.Add(1)+go func(){defer wg.Done()...}()` 块 |
| 10 | `sync_waitgroup_go` | Getter 接口重构：`Get2ChanWG(pc, wg)` 去掉 `wg` 参数与内部 `defer wg.Done()`，调用点改用 `wg.Go` | pkg/getter/base.go:15（接口定义）+ 6 个实现（subscribe/clash/tgchannel/web_fuzz_sub/web_free_ssr_xyz/web_fanqiangdang）+ task.go 调用点 |
| 11 | 并发简化 | anytls_probe.go 中 `wg + workerpool.Submit` 的混合等待 → 利用 `workerpool.StopWait()` 去掉独立 wg（每个 index 只写一次，结果切片并发安全），同时删除冗余循环变量拷贝 | internal/app/anytls_probe.go:116-125 |

### P3：顺手修正（非规范驱动，低风险）

| # | 改动 | 位置 |
|---|------|------|
| 12 | 无参数 `fmt.Errorf` → `errors.New`，并修正拼写 "invaild" → "invalid" | internal/app/best_proxy_ip.go:777, 805 |

### 可选（默认不做，待确认）

- `json_omitzero`：vless.go/trojan.go/anytls.go 中 bool 字段 `json:"...,omitempty"` → `omitzero`（bool/数值零值省略语义与 omitempty 完全一致，行为安全但收益低）。
- `err == nil` 反向条件改早退风格：非规范驱动，纯风格，改动面大，建议跳过。

### 已排查、不适用

`reflect.TypeOf((*T)(nil)).Elem()`、`b.N` benchmark、测试中 `t.Context()`、`time.Tick`、手写 min/max、`err == target`、`errors.Join`、`sync.Once`、`maps/slices` 缺失（openaicheck.go 已用 slices）、`time.Until/Since`、`context.AfterFunc`。

## 三、实施步骤与验证

每个阶段独立提交，提交前在 Docker 内验证：

```sh
docker run --rm -v "$PWD":/app -w /app golang:1.27.0-alpine \
  sh -c "apk add --no-cache make git && GOPROXY=https://goproxy.cn,direct \
         gofmt -l . && go vet ./... && go build ./... && go test ./... 2>&1 | tail -30"
```

提交拆分建议（每步一个 commit，`refactor: modern-go: <主题>`）：

1. `any` 全量替换（P0-1）
2. strings.Cut / CutLast（P0-2,3）
3. range_over_int（P0-4）
4. config atomic.Pointer（P1-5）
5. loopvar 清理 + slices.SortStableFunc（P1-6,7,8）
6. Getter 接口去 wg + wg.Go + anytls_probe 简化（P2-9,10,11）
7. errors.New / 拼写修正（P3-12）

## 四、风险与说明

- **P2-10 接口签名变更**会触及 getter 包全部实现，属结构性改动；行为等价（wg.Done 语义由 wg.Go 内部完成），但需重点回归抓取任务。
- **P1-8 排序重写**中 best_proxy_ip 的国家/IP/端口多级比较逻辑复杂（含 IPv4 数值比较），转换时逐分支等价改写，避免改变排序结果。
- 全程不动第三方依赖版本，不引入新依赖（slices/maps/cmp 均为标准库）。
- 最终用 `git diff` 复查，确保除目标模式外无其他改动。
