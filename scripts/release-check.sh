#!/usr/bin/env bash
# 发版前门禁：把"能不能发"这件事变成一条命令。
#
# 覆盖：构建 → 静态检查 → 单元/集成测试 → 竞态 → 前端构建 → API 面对齐
#       → 端到端（四条传输等） → 前端渲染/点击。
#
# 用法：
#   ./scripts/release-check.sh            # 全跑
#   SKIP_UI=1 ./scripts/release-check.sh  # 跳过前端渲染（无 chromium 的环境）
#   SKIP_E2E=1 ./scripts/release-check.sh # 跳过端到端（时间紧时）
set -uo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT" || exit 1

declare -a NAMES=() RESULTS=() DURATIONS=()
FAILED=0

run() { # run <名称> <命令...>
  local name=$1; shift
  local start=$SECONDS
  printf '\n\033[36m=== %s\033[0m\n' "$name"
  if "$@"; then
    RESULTS+=("PASS")
  else
    RESULTS+=("FAIL"); FAILED=1
  fi
  NAMES+=("$name")
  DURATIONS+=("$((SECONDS - start))s")
}

run "服务端构建"   bash -c 'go build ./...'
run "静态检查"     bash -c 'go vet ./...'
run "服务端测试"   bash -c 'go test ./... -count=1'
run "竞态检测"     bash -c 'go test -race ./c2engine ./controllers ./router ./mcp ./utils -count=1'
run "agent 构建/测试" bash -c 'cd agent && go build ./... && go test ./... -count=1'
run "前端构建"     bash -c 'cd web && pnpm run build'
run "API 面对齐"   bash -c 'python3 scripts/api-parity.py | grep -q "缺失 0 个"'

[[ "${SKIP_E2E:-0}" == "1" ]] || run "端到端 (e2e.sh)" bash -c './scripts/e2e.sh >/tmp/release-e2e.log 2>&1; tail -1 /tmp/release-e2e.log | grep -q PASS'
[[ "${SKIP_UI:-0}"  == "1" ]] || run "前端渲染/点击 (ui-check.sh)" bash -c './scripts/ui-check.sh >/tmp/release-ui.log 2>&1; grep -q "全部渲染" /tmp/release-ui.log'

printf '\n\033[1m=== 汇总 ===\033[0m\n'
for i in "${!NAMES[@]}"; do
  if [[ "${RESULTS[$i]}" == "PASS" ]]; then
    printf '  \033[32m✓\033[0m %-28s %s\n' "${NAMES[$i]}" "${DURATIONS[$i]}"
  else
    printf '  \033[31m✗\033[0m %-28s %s\n' "${NAMES[$i]}" "${DURATIONS[$i]}"
  fi
done

if [[ $FAILED -eq 0 ]]; then
  printf '\n\033[32m全部门禁通过，可以发版。\033[0m\n'
  exit 0
fi
printf '\n\033[31m有门禁未通过，先修再发。\033[0m\n'
exit 1
