#!/bin/sh
# Modern Go 优化验证：gofmt + vet + build + test（Docker 内执行）
# 用法: sh scripts/verify-go.sh [test|vet|build|fmt|all]
set -e
cd "$(dirname "$0")/.."
MODE="${1:-all}"

VOL="gomodcache-pp"
# 预热共享模块缓存卷
docker volume create "$VOL" >/dev/null 2>&1 || true

run() {
  docker run --rm -v "$VOL":/go/pkg/mod -v "$PWD":/app -w /app golang:1.27.0-alpine \
    sh -c "export GOPROXY=https://goproxy.cn,direct; $1"
}

case "$MODE" in
  fmt)   run "gofmt -l . | grep -v vendor || echo 'fmt OK'" ;;
  vet)   run "go vet ./... 2>&1 | tail -20 || echo 'VET FAILED'" ;;
  build) run "go build ./... && echo 'BUILD OK'" ;;
  test)  run "go test ./... 2>&1 | tail -40" ;;
  all)
    echo "=== gofmt ==="
    run "gofmt -l . | grep -v vendor || echo 'fmt OK'"
    echo "=== go vet ==="
    run "go vet ./... 2>&1 | tail -20"
    echo "=== go build ==="
    run "go build ./... && echo 'BUILD OK'"
    echo "=== go test ==="
    run "go test ./... 2>&1 | tail -40"
    ;;
esac
