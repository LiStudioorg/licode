#!/usr/bin/env bash
# 重新生成前端静态产物（web/dist）并同步到 internal/web/dist（go:embed 目录）。
#
# 前端为 web/ 下的 Vite + Vue3 + TS + Pinia + Tailwind 工程。
# 默认跳过 node 构建，直接使用已入库的 internal/web/dist 产物
# （go build 始终可用，不依赖 node）；只有显式 LICODE_FRONTEND_BUILD=1
# 时才真正执行 vite 构建。
set -euo pipefail
cd "$(dirname "$0")/.."

if [ "${LICODE_FRONTEND_BUILD:-0}" != "1" ]; then
    echo "[web] 默认跳过（构建需 node；如需重建请显式 LICODE_FRONTEND_BUILD=1）" >&2
    exit 0
fi

if ! command -v node >/dev/null 2>&1; then
    echo "[web] node 不可用，跳过（使用已提交的 internal/web/dist 产物）" >&2
    exit 0
fi

if [ ! -d "web" ]; then
    echo "[web] 未找到 web/，跳过" >&2
    exit 0
fi

if [ ! -d "web/node_modules" ]; then
    echo "[web] web/node_modules 不存在，跳过重建（如需重建先 cd web && npm install）" >&2
    exit 0
fi

echo "==> vite build ..."
( cd web && npx vite build )

if [ ! -f "web/dist/index.html" ]; then
    echo "[web] 构建产物缺少 web/dist/index.html，放弃同步" >&2
    exit 1
fi

echo "==> 同步 web/dist -> internal/web/dist ..."
rm -rf internal/web/dist
mkdir -p internal/web/dist
cp -r web/dist/. internal/web/dist/

# .gitkeep 保证 embed 目标目录在产物被清空时仍然存在
rm -f internal/web/dist/.gitkeep

echo "完成：internal/web/dist/ 共 $(find internal/web/dist -type f | wc -l) 个文件"
