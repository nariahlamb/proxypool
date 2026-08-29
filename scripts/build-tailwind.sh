#!/bin/sh
# 本地编译 Tailwind CSS：预编译产物入库（config/assets/static/css/index.css），
# Docker/CI 构建无需 Node —— 改样式后重跑本脚本并提交产物即可。
# 用法：sh scripts/build-tailwind.sh
set -e
cd "$(dirname "$0")/.."

docker run --rm \
  -v gomodcache-pp:/go/pkg/mod \
  -v tw-node-modules:/app/node_modules \
  -v "$PWD":/app -w /app node:22-alpine \
  sh -c "npm install --no-save --no-package-lock tailwindcss @tailwindcss/cli >/dev/null 2>&1 && npx @tailwindcss/cli -i config/assets/static/css/tailwind-input.css -o config/assets/static/css/index.css --minify"

# npm 可能在工作目录生成 package.json，移除（构建自包含，依赖走 node_modules 卷）
rm -f package.json package-lock.json

echo "✓ Tailwind 编译完成: config/assets/static/css/index.css"
