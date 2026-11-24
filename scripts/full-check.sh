#!/usr/bin/env bash
# 全量回归：发版门禁 + 端到端 + 前端点击级 + MCP 冒烟
#
# 用法：
#   ./scripts/full-check.sh              # 全跑
#   FAST=1 ./scripts/full-check.sh       # 跳过 e2e 与 ui-check（只跑构建/单测/门禁）
#   REL=~/vshell-release ./scripts/full-check.sh   # 指定 MCP 冒烟用的服务端目录
set -uo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 1

REL_DIR=${REL:-$HOME/vshell-release}
start=$(date +%s)
declare -a NAMES RESULTS
run() { # run <名称> <命令...>
  local name=$1; shift
  local t0=$(date +%s)
  if "$@" >/tmp/full-check-step.log 2>&1; then
    RESULTS+=("ok"); printf '  \033[32m✓\033[0m %-28s %ss\n' "$name" "$(( $(date +%s) - t0 ))"
  else
    RESULTS+=("fail"); printf '  \033[31m✗\033[0m %-28s %ss（日志 /tmp/full-check-step.log）\n' "$name" "$(( $(date +%s) - t0 ))"
    tail -5 /tmp/full-check-step.log | sed 's/^/      /'
  fi
  NAMES+=("$name")
}

echo "=== 1/4 发版门禁 ==="
if [[ "${FAST:-0}" == "1" ]]; then
  SKIP_E2E=1 SKIP_UI=1 run "release-check(跳过 e2e/ui)" ./scripts/release-check.sh
else
  SKIP_E2E=1 SKIP_UI=1 run "release-check" ./scripts/release-check.sh
fi

if [[ "${FAST:-0}" != "1" ]]; then
  echo "=== 2/4 端到端（HTTP/KCP/WS/DNS/TCP + 面板契约 + 载荷）==="
  run "e2e" ./scripts/e2e.sh
  echo "=== 3/4 前端点击级（12 页 + 13 条交互）==="
  run "ui-check" ./scripts/ui-check.sh
else
  echo "=== 2-3/4 跳过 e2e / ui-check（FAST=1）==="
fi

echo "=== 4/4 原生 MCP 冒烟 ==="
run "mcp-smoke" ./scripts/mcp-smoke.sh "$REL_DIR"

echo
echo "=== 汇总（总耗时 $(( $(date +%s) - start ))s）==="
fail=0
for i in "${!NAMES[@]}"; do
  printf '  %-24s %s\n' "${NAMES[$i]}" "${RESULTS[$i]}"
  [[ "${RESULTS[$i]}" == "fail" ]] && fail=$((fail+1))
done
if [[ $fail -eq 0 ]]; then
  printf '\n\033[32m全部通过\033[0m\n'
  exit 0
fi
printf '\n\033[31m有 %d 项失败\033[0m\n' "$fail"
exit 1
