<h1 align="center">
  <br>proxypool<br>
</h1>

<h5 align="center">自动抓取 tg 频道、订阅地址、公开互联网上的 ss、ssr、vmess、trojan、vless、anytls 节点信息，聚合去重测试可用性后提供节点列表与 Cloudflare 优选 IP 订阅</h5>

<p align="center">
  <a href="https://github.com/One-Piecs/proxypool/actions">
    <img src="https://img.shields.io/github/actions/workflow/status/One-Piecs/proxypool/docker-release.yml?style=flat-square" alt="Github Actions">
  </a>
  <a href="https://goreportcard.com/report/github.com/One-Piecs/proxypool">
    <img src="https://goreportcard.com/badge/github.com/One-Piecs/proxypool?style=flat-square">
  </a>
  <a href="https://github.com/One-Piecs/proxypool/tags">
    <img src="https://img.shields.io/github/v/tag/One-Piecs/proxypool.svg?style=flat-square">
  </a>
</p>

## 功能特性

- 支持 ss、ssr、vmess、trojan、vless、anytls 多种节点类型
- Telegram 频道抓取、订阅地址抓取解析、公开互联网页面模糊抓取
- 定时抓取自动更新，自动检测节点可用性（延迟/测速/中转/OpenAI 检测）
- 节点去重、按国家聚合、测速结果持久化（SQLite）
- 提供 clash、surge、shadowrocket、loon、quanx 配置
- 提供 ss、ssr、vmess、trojan、vless、sip002 订阅
- **Cloudflare 优选 IP**：多源采集（Top20/ISP/明文订阅）、多级排序、SNI 透传探测（sni_probe），输出各客户端格式优选订阅

## 安装

以下三选一。

### 1. 从源码编译

需要安装 Golang 1.27+：

```sh
$ git clone https://github.com/One-Piecs/proxypool.git
$ cd proxypool
$ go build -o proxypool .
```

运行：

```sh
$ ./proxypool -c ./config/config.yaml
```

交叉编译全部平台：

```sh
$ make
```

### 2. 使用 Docker

```sh
docker build -t proxypool .
docker run -d -p 12580:12580 -v $PWD/config:/app/config -v $PWD/data:/app/data proxypool -d -c config/config.yaml
```

### 3. 使用发布版本（Docker 镜像）

项目不提供独立二进制下载。打 tag（如 `v1.1.61`）后，GitHub Actions（`docker-release.yml`）自动构建 **linux/amd64、linux/arm64** 的 Docker 镜像并推送 Docker Hub（`<DOCKERHUB_USERNAME>/proxypool`，tag 与 `latest`）。

```sh
docker pull <dockerhub用户名>/proxypool:latest
```

历史版本号见 [CHANGELOG.md](CHANGELOG.md) 与 [Git tags](https://github.com/One-Piecs/proxypool/tags)。

## 使用

运行该程序需要具有访问完整互联网的能力。默认监听端口 **12580**（可用 `port` 配置项修改）。

### 配置文件

修改 `config/config.yaml`（或 `source.yaml` 中的抓取源）。带默认值的字段均可不填，关键配置项：

| 配置 | 说明 |
|------|------|
| `domain` / `port` | 站点域名 / 监听端口（默认 12580） |
| `source-files` | 抓取源列表文件（支持本地文件与 http 链接） |
| `speedtest` / `speedtest-interval` | 测速开关 / 间隔 |
| `sub-best-node-interval` | 优选 IP 任务刷新间隔（默认 60 min） |
| `sub_ip_url` | Cloudflare 优选 IP 订阅源（域名/IP 列表） |
| `sub_ip_list_url` | 明文 `ip:port#国家` 订阅源（端口白名单 443/2053/2083/2087/2096/8443，纯 IP 行默认补 443） |
| `sni_probe` | 优选节点 SNI 透传探测（enable/concurrency/timeout/country/test_url） |
| `healthcheck_test_urls` | 健康检查测试地址覆盖（默认内置国内可达 204 端点，不含 gstatic） |
| `proxy_info` | 优选 IP 出站节点凭据（vmess/trojan/vless/anytls，按国家） |
| `cf_best_ip` | 静态优选 IP 列表 |

`source.yaml` 中的 getter 类型：`subscribe`、`clash`、`webfuzz`、`webfuzzsub`、`tgchannel`、`web-fanqiangdang`、`web-freessrxyz` 等。

## API 文档

以下均为 `GET` 请求，`[端口]` 为监听端口。

### 页面

| 路径 | 说明 |
|------|------|
| `/` | 首页 |
| `/clash` | Clash 配置页面 |
| `/surge` | Surge 配置页面 |
| `/shadowrocket` | Shadowrocket 配置页面 |
| `/loon` | Loon 配置页面 |
| `/quanx` | Quantumult X 配置页面 |

### 客户端配置

| 路径 | 说明 |
|------|------|
| `/clash/config` | Clash 远程配置 |
| `/clash/localconfig` | Clash 本地配置（127.0.0.1） |
| `/surge/config` | Surge 配置 |
| `/clash/proxies` | Clash 节点列表（YAML） |
| `/surge/proxies` | Surge 节点列表 |
| `/loon/proxies` | Loon 节点列表 |
| `/v2rayn/proxies` | v2rayN 节点列表 |
| `/quanx/proxies` | Quantumult X 节点列表 |

### 订阅

| 路径 | 说明 |
|------|------|
| `/ss/sub`、`/sip002/sub` | SS / SIP002 订阅 |
| `/ssr/sub` | SSR 订阅 |
| `/vmess/sub`、`/vless/sub`、`/trojan/sub` | vmess / vless / trojan 订阅 |
| `/link/:id` | 按 ID 查看单个节点 |

### Cloudflare 优选 IP

`/best*` 端点，`:format` 为客户端+类型组合（如 `clashVmess`、`surgeTrojan`、`quanxVless`、`loonAnytls`、`v2raynTrojan`、`clashAnytls` 等），支持参数 `country`、`limit`、`random`、`cdn`、`ipv6`：

| 路径 | 说明 |
|------|------|
| `/bestProxyIp/:format` | 全部优选 IP 节点 |
| `/bestCfProxyIp/:format` | Cloudflare 优选 IP |
| `/bestCfProxyIpTop20/:format` | CF Top20 优选 |
| `/bestCfProxyIpIsp/:format` | CF 各 ISP（电信/联通/移动）优选 |
| `/bestCfProxyDomainTop20/:format` | CF 域名 Top20 优选 |
| `/bestCfProxySub/:format` | CF 明文订阅源优选 |
| `/bestIpKr/:format` | 韩国优选 |

### 任务与运维

| 路径 | 说明 |
|------|------|
| `/task/crawl` | 手动触发抓取 |
| `/task/speedtest` | 手动触发测速 |
| `/task/updateGeoIP` | 更新 GeoIP 数据库 |
| `/task/updateBestNode` | 手动刷新优选 IP |
| `/health` | 健康检查 |
| `/debug/statsviz/*filepath` | [statsviz](https://github.com/arl/statsviz) 运行时指标 |

## 本地检查节点可用性

此项非必须。为了提高实际可用性，可选择增加一个本地服务器，检测远程 proxypool 节点在本地的可用性并提供配置，见 [proxypoolCheck](https://github.com/Sansui233/proxypoolCheck)。

## 声明

本项目遵循 GNU General Public License v3.0 开源，在此基础上，所有使用本项目提供服务者都必须在网站首页保留指向本项目的链接。

本项目仅限个人自己使用，禁止使用本项目进行营利和做其他违法事情，产生的一切后果本项目概不负责。
