#!/usr/bin/env bash
# 兼容保留：前端已从 Nuxt 迁移到 Vite + Vue3 + Vueless UI，
# 实际构建逻辑见 scripts/sync-frontend.sh。此脚本仅做转发，
# 避免既有调用方（脚本/文档/CI）失效。
set -euo pipefail
exec bash "$(dirname "$0")/sync-frontend.sh"
