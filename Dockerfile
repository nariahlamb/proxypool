# 构建阶段
ARG GO_VERSION=1.27.0
FROM golang:${GO_VERSION}-alpine AS builder

# 安装构建依赖并清理缓存
RUN apk add --no-cache make git \
    && rm -rf /var/cache/apk/*

WORKDIR /proxypool-src
COPY . .
# 使用国内镜像加速依赖下载
ENV GOPROXY=https://goproxy.cn,direct
RUN make docker && mv bin/proxypool-docker /proxypool

# ----------------------------
# 运行阶段
FROM alpine:3.24

# 安装运行时依赖并清理缓存
RUN apk add --no-cache ca-certificates tzdata \
    && rm -rf /var/cache/apk/*

# 创建文件夹
RUN mkdir -p /app/config /app/data

WORKDIR /app

# 创建文件夹
RUN mkdir -p /app/assets

# 复制配置文件（显式创建目录）
COPY ./config/config.yaml ./config/source.yaml /app/config/
# 复制 GeoIP 数据库（运行时文件，非 embed）：缺失时启动 InitGeoIpDB 失败会 os.Exit(1)
COPY --from=builder /proxypool-src/assets/Country.mmdb /proxypool-src/assets/GeoLite2-ASN.mmdb /proxypool-src/assets/version /app/assets/
COPY --from=builder /proxypool /app/

# 设置时区
ENV TZ=Asia/Shanghai

# 暴露端口
EXPOSE 12580

# 健康检查（alpine 自带 busybox wget）
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -q -O- http://localhost:12580/health || exit 1

# 启动命令（使用 CMD 允许覆盖参数）
ENTRYPOINT ["/app/proxypool"]
CMD ["-d", "-c", "config/config.yaml"]

# 添加元数据标签（可选）
LABEL maintainer="proxypool@laibas.top" \
    description="ProxyPool Service"