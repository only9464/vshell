#!/usr/bin/env bash
# 原生 MCP 冒烟：initialize → tools/list → dashboard → run_command（真实客户端）
# 用法：./scripts/mcp-smoke.sh [服务端目录，默认 ~/vshell-release]
set -uo pipefail
DIR=${1:-$HOME/vshell-release}
cd "$DIR" || { echo "目录不存在: $DIR"; exit 1; }

OUT=$(printf '%s\n%s\n%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' \
  '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"dashboard","arguments":{}}}' \
  '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"run_command","arguments":{"client_id":1,"command":"echo mcp-smoke-ok"}}}' \
  | ./vshell-server mcp 2>/dev/null)

python3 - "$OUT" <<'PY'
import json, sys
raw = sys.argv[1]
tools, dash, run = [], None, None
for line in raw.splitlines():
    line = line.strip()
    if not line:
        continue
    try:
        d = json.loads(line)
    except Exception:
        continue
    r = d.get("result") or {}
    if "tools" in r:
        tools = [t["name"] for t in r["tools"]]
    txt = "".join(c.get("text", "") for c in r.get("content", []) or [])
    if d.get("id") == 3:
        dash = txt
    if d.get("id") == 4:
        run = txt
ok = True
if len(tools) != 13:
    print(f"[FAIL] tools/list 期望 13 个，实际 {len(tools)}"); ok = False
else:
    print(f"[ok] tools/list: {len(tools)} 个工具")
if not dash or '"clientCount"' not in dash:
    print(f"[FAIL] dashboard 未返回计数: {(dash or '')[:80]}"); ok = False
else:
    print("[ok] dashboard 返回 clientCount")
if not run or 'mcp-smoke-ok' not in run:
    print(f"[FAIL] run_command 未拿到回显: {(run or '')[:80]}"); ok = False
else:
    print("[ok] run_command 在真实客户端上拿到回显")
sys.exit(0 if ok else 1)
PY
