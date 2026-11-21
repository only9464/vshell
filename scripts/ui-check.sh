#!/usr/bin/env bash
# 前端冒烟检查：无头 Chromium 渲染 Vue3 面板的关键页面，断言真实数据出现。
#
# 做法：起服务端 + 一个 agent → 生成一个"带 token 的 bootstrap 页面"（同源写
# localStorage 后跳转到目标路由）→ chromium --dump-dom → 断言 DOM 里出现关键内容。
#
# 为什么需要它：后端接口测试全绿，也可能因为字段大小写不一致导致表格整列空白
# （R45 就发现了 file 列表 Name/name 不匹配）。纯接口测试看不出来，必须真渲染。
#
# 用法：./scripts/ui-check.sh    （要求 chromium 存在、web/dist 已构建）
set -uo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
TMP=$(mktemp -d "$HOME/.vshell-ui.XXXXXX")
WEB_PORT=${WEB_PORT:-18171}
LISTEN_PORT=${LISTEN_PORT:-18172}
CHROME=${CHROME:-chromium}
SERVER_PID=""; AGENT_PID=""

cleanup() {
  [[ -n "$AGENT_PID"  ]] && kill "$AGENT_PID"  2>/dev/null
  [[ -n "$SERVER_PID" ]] && kill "$SERVER_PID" 2>/dev/null
  rm -rf "$TMP"
}
trap cleanup EXIT

log() { echo "[ui] $*"; }
fail() { echo "[ui] FAIL: $*" >&2; exit 1; }

command -v "$CHROME" >/dev/null || fail "找不到 $CHROME"
[[ -f "$ROOT/web/dist/index.html" ]] || fail "web/dist 未构建（cd web && pnpm build）"

mkdir -p "$TMP/conf" "$TMP/db"
ln -sfn "$ROOT/static" "$TMP/static"
ln -sfn "$ROOT/web" "$TMP/web"
cat > "$TMP/conf/setting.conf" <<CONF
master_type=web
web_port=$WEB_PORT
web_ip=127.0.0.1
web_basic_auth=false
web_jwt_secret=ui-check
web_username=admin
web_password=p1
CONF

( cd "$ROOT" && go build -o "$TMP/vshell-server" . ) || fail "服务端构建失败"
( cd "$TMP" && exec ./vshell-server > server.log 2>&1 ) &
SERVER_PID=$!
for _ in $(seq 1 50); do curl -s -o /dev/null -m 1 "http://127.0.0.1:$WEB_PORT/" && break; sleep 0.2; done

TOK=$(curl -s -X POST "http://127.0.0.1:$WEB_PORT/api/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"p1"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["token"])')
[[ -n "$TOK" ]] || fail "登录失败"

curl -s -X POST "http://127.0.0.1:$WEB_PORT/api/listener/add" -H "Token: $TOK" \
  -H 'Content-Type: application/json' \
  -d "{\"Mode\":\"http\",\"Vkey\":\"UI-VK\",\"Host\":\"127.0.0.1\",\"Port\":$LISTEN_PORT,\"PingInterval\":1}" >/dev/null
( cd "$ROOT/agent" && go build -ldflags "-X main.ServerAddr=http://127.0.0.1:$LISTEN_PORT -X main.VerifyKey=UI-VK -X main.EncryptSalt=" -o "$TMP/agent" . ) \
  || fail "agent 构建失败"
( cd "$TMP" && exec ./agent > agent.log 2>&1 ) &
AGENT_PID=$!
for _ in $(seq 1 40); do
  COUNT=$(curl -s -X POST "http://127.0.0.1:$WEB_PORT/api/client/list" -H "Token: $TOK" \
    -H 'Content-Type: application/json' -d '{"page":1,"pageSize":5}' \
    | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["clientOnlineCount"])' 2>/dev/null || echo 0)
  [[ "$COUNT" -ge 1 ]] && break
  sleep 0.5
done
[[ "${COUNT:-0}" -ge 1 ]] || fail "agent 未上线"

# 渲染一个路由并返回 DOM
render() { # $1=route  $2=输出文件
  cat > "$TMP/web/dist/bootstrap.html" <<BOOT
<!doctype html><html><head><meta charset="utf-8"></head><body><script>
localStorage.setItem('vshell.token','$TOK');
localStorage.setItem('vshell.username','admin');
location.replace('/app/$1');
</script></body></html>
BOOT
  timeout 90 "$CHROME" --headless=new --disable-gpu --no-sandbox \
    --virtual-time-budget=15000 --dump-dom "http://127.0.0.1:$WEB_PORT/app/bootstrap.html" \
    > "$2" 2>/dev/null
}

expect() { # $1=文件 $2=关键字 $3=说明
  grep -q -- "$2" "$1" || fail "$3（DOM 里没有 $2）"
}

log "渲染 #/dashboard"
render '#/dashboard' "$TMP/dash.html"
expect "$TMP/dash.html" '运行信息' 'dashboard 未渲染'
expect "$TMP/dash.html" '版本' 'dashboard 缺少 KPI'

log "渲染 #/clients"
render '#/clients' "$TMP/clients.html"
expect "$TMP/clients.html" "$(hostname)" '客户端列表没有主机名（字段契约不符？）'
expect "$TMP/clients.html" '在线' '客户端列表没有在线状态'

log "渲染 #/files/1"
render '#/files/1' "$TMP/files.html"
expect "$TMP/files.html" '修改时间' '文件页表头缺失'
python3 - "$TMP/files.html" <<'PY' || fail "文件列表为空（file/ls 字段大小写不匹配？）"
import re,sys
s=open(sys.argv[1],encoding='utf-8',errors='ignore').read()
rows=re.findall(r'<td[^>]*>(.*?)</td>', s, re.S)
cells=[re.sub(r'<[^>]+>','',c).strip() for c in rows]
if not any(c.endswith('/') or c.endswith('.txt') or c for c in cells[:4]):
    raise SystemExit(1)
PY

log "渲染 #/terminal/1"
render '#/terminal/1' "$TMP/term.html"
expect "$TMP/term.html" 'xterm' '终端 xterm 未挂载'
expect "$TMP/term.html" '已连接' '终端 WebSocket 未连上'

log "全部页面渲染正常（dashboard / clients / files / terminal）"
