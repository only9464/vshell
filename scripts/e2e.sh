#!/usr/bin/env bash
# 端到端冒烟测试：起服务 → 登录 → 建监听器 → 编译 agent → 上线 → 执行命令并拿到回显。
#
# 设计约束：
#   - 不需要 root（全部用高位端口）；
#   - 不污染仓库（服务端在临时目录里跑，static/ 用软链接）；
#   - 失败时打印两端日志并返回非 0。
#
# 用法：scripts/e2e.sh [-v]
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERBOSE=0
[[ "${1:-}" == "-v" ]] && VERBOSE=1

WEB_PORT=${WEB_PORT:-$((18000 + RANDOM % 1000))}
LST_PORT=${LST_PORT:-$((19000 + RANDOM % 1000))}
VKEY="vk_e2e_$$"
USER_NAME=admin
PASS="e2e_$(date +%s)"
TMP="$(mktemp -d /tmp/vshell-e2e.XXXXXX)"

SERVER_PID=""
AGENT_PID=""
AGENT_KCP_PID=""
AGENT_WS_PID=""
AGENT_DNS_PID=""
AGENT_PAY_PID=""
OTCP_PID=""
TGT_PID=""

log()  { printf '\033[36m[e2e]\033[0m %s\n' "$*"; }
fail() {
  printf '\033[31m[e2e] FAIL:\033[0m %s\n' "$*"
  echo "--- server.log (tail) ---"; tail -n 80 "$TMP/server.log" 2>/dev/null
  echo "--- server.log (DNS) ---"; grep -E "DNS" "$TMP/server.log" 2>/dev/null | tail -20
  for f in "$TMP"/*.log; do
    [[ "$(basename "$f")" == "server.log" ]] && continue
    echo "--- $(basename "$f") (tail) ---"; tail -n 30 "$f" 2>/dev/null
  done
  cleanup
  exit 1
}

cleanup() {
  [[ -n "$AGENT_PID"     ]] && kill "$AGENT_PID"     2>/dev/null
  [[ -n "$AGENT_KCP_PID" ]] && kill "$AGENT_KCP_PID" 2>/dev/null
  [[ -n "$AGENT_WS_PID"  ]] && kill "$AGENT_WS_PID"  2>/dev/null
  [[ -n "$AGENT_DNS_PID" ]] && kill "$AGENT_DNS_PID" 2>/dev/null
  [[ -n "$AGENT_PAY_PID" ]] && kill "$AGENT_PAY_PID" 2>/dev/null
  [[ -n "$TGT_PID"      ]] && kill "$TGT_PID"      2>/dev/null
  [[ -n "$OTCP_PID"     ]] && kill "$OTCP_PID"     2>/dev/null
  [[ -n "$SERVER_PID"    ]] && kill "$SERVER_PID"    2>/dev/null
  rm -rf "$TMP"
}
trap cleanup EXIT

# jget <json> <python 表达式，用 d 表示解析后的对象>
jget() { python3 -c 'import sys,json;d=json.load(sys.stdin);print(eval(sys.argv[1]))' "$2" <<<"$1"; }

api() { # api <method> <path> [json]
  local method=$1 path=$2 body=${3:-}
  if [[ -n "$body" ]]; then
    curl -sS -m 10 -X "$method" "http://127.0.0.1:$WEB_PORT/api$path" \
      -H "Token: $TOKEN" -H 'Content-Type: application/json' -d "$body"
  else
    curl -sS -m 10 -X "$method" "http://127.0.0.1:$WEB_PORT/api$path" -H "Token: $TOKEN"
  fi
}

# ---------------------------------------------------------------- 1. build
log "build server + agent (root: $ROOT)"
( cd "$ROOT" && go build -o "$TMP/vshell-server" . ) || fail "server build"
( cd "$ROOT/agent" && go build -o "$TMP/agent" . ) || fail "agent build"
ln -s "$ROOT/static" "$TMP/static"
# 新版面板产物（若已构建）也挂进来，供 /app/ 托管检查
[[ -d "$ROOT/web" ]] && ln -s "$ROOT/web" "$TMP/web"
mkdir -p "$TMP/conf" "$TMP/db"

cat > "$TMP/conf/setting.conf" <<EOF
master_type=web
web_title=VShell-E2E
web_port=$WEB_PORT
web_ip=127.0.0.1
web_basic_auth=false
web_jwt_secret=e2e-secret
web_username=$USER_NAME
web_password=$PASS
log_level=7
EOF

# ---------------------------------------------------------------- 2. server
log "start server on :$WEB_PORT"
# 用 exec 让子 shell 被服务端进程替换，$! 即服务端 PID（非交互 bash 的
# 子 shell 会等待其后台作业，不能用 `( cmd & )` 的写法）。
( cd "$TMP" && exec ./vshell-server > server.log 2>&1 ) &
SERVER_PID=$!
[[ -n "$SERVER_PID" ]] || fail "server did not start"

READY=0
for _ in $(seq 1 50); do
  if curl -s -o /dev/null -m 1 "http://127.0.0.1:$WEB_PORT/"; then READY=1; break; fi
  kill -0 "$SERVER_PID" 2>/dev/null || break
  sleep 0.2
done
[[ $READY -eq 1 ]] || fail "server did not become reachable"

# ---------------------------------------------------------------- 3. login
log "login"
RESP=$(curl -sS -m 5 -X POST "http://127.0.0.1:$WEB_PORT/api/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER_NAME\",\"password\":\"$PASS\"}")
TOKEN=$(jget "$RESP" 'd["token"]' 2>/dev/null || jget "$RESP" 'd["result"]["token"]' 2>/dev/null || true)
[[ -n "${TOKEN:-}" ]] || fail "login failed: $RESP"

# ---------------------------------------------------------------- 4. listener
log "create http listener on 127.0.0.1:$LST_PORT"
api POST /listener/add "{\"Mode\":\"http\",\"Vkey\":\"$VKEY\",\"Remark\":\"e2e\",\"Host\":\"127.0.0.1\",\"Port\":$LST_PORT,\"PingInterval\":1}" >/dev/null

LST=$(api POST /listener/list '{"page":1,"pageSize":10}')
LST_ID=$(jget "$LST" 'd["result"]["items"][0]["Id"]' 2>/dev/null || true)
SALT=$(jget "$LST" 'd["result"]["items"][0]["EncryptSalt"]' 2>/dev/null || true)
[[ -n "${LST_ID:-}" ]] || fail "listener not created: $LST"
# 原版 listener/add 的 EncryptSalt 缺省为空（帧密钥 = md5("")），因此这里不再
# 要求非空；agent 用同一取值即可。
log "listener id=$LST_ID salt=${SALT:-<empty>}"

START_RESP=$(api POST /listener/start "{\"id\":$LST_ID}")
echo "$START_RESP" | grep -q '"code":0' || echo "$START_RESP" | grep -q 'already start' \
  || fail "listener start failed: $START_RESP"
log "listener started (add auto-starts it, like the original)"

# ---------------------------------------------------------------- 5. agent
log "build agent with injected config"
( cd "$ROOT/agent" && go build -ldflags "\
  -X main.ServerAddr=http://127.0.0.1:$LST_PORT \
  -X main.VerifyKey=$VKEY \
  -X main.EncryptSalt=$SALT" -o "$TMP/agent" . ) || fail "agent build with ldflags"

log "start agent"
"$TMP/agent" > "$TMP/agent.log" 2>&1 &
AGENT_PID=$!
sleep 0.2
kill -0 "$AGENT_PID" 2>/dev/null || fail "agent exited immediately"

# ---------------------------------------------------------------- 6. wait online
CLIENT_ID=""
for _ in $(seq 1 60); do
  LIST=$(api POST /client/list '{"page":1,"pageSize":10}')
  CLIENT_ID=$(jget "$LIST" 'd["result"]["items"][0]["Id"] if d["result"]["items"] else 0' 2>/dev/null || echo 0)
  ONLINE=$(jget "$LIST" 'd["result"]["clientOnlineCount"]' 2>/dev/null || echo 0)
  [[ "$ONLINE" != "0" && -n "$CLIENT_ID" && "$CLIENT_ID" != "0" ]] && break
  sleep 0.5
done
[[ "${ONLINE:-0}" != "0" ]] || fail "agent never came online (see STATUS §4-①②)"
log "client $CLIENT_ID online"

# ---------------------------------------------------------------- 7. run command
log "run 'id' through /terminal/shell"
OUT=$(api POST /terminal/shell "{\"id\":$CLIENT_ID,\"command\":\"id\"}")
echo "$OUT" | grep -q '"code":0' || fail "terminal/shell failed: $OUT"
TEXT=$(jget "$OUT" 'd["result"]["output"]' 2>/dev/null || true)
echo "$OUT" | grep -qE 'uid=[0-9]+' || fail "unexpected output: $OUT"
[[ $VERBOSE -eq 1 ]] && printf '\033[32moutput:\033[0m %s\n' "$TEXT"

# ---------------------------------------------------------------- 7b. 文件上传（B5.2）
log "check /api/file/upload (multipart -> agent)"
UP_TOKEN="upload-check-$$"
printf '%s\n' "$UP_TOKEN" > "$TMP/up.txt"
UP_RESP=$(curl -sS -m 30 -X POST "http://127.0.0.1:$WEB_PORT/api/file/upload" \
  -H "Token: $TOKEN" \
  -F "id=$CLIENT_ID" -F "path=/tmp" -F "file=@$TMP/up.txt")
echo "$UP_RESP" | grep -q '"code":0' || fail "upload failed: $UP_RESP"
# 上传是异步下发（agent 轮询取任务），这里轮询直到能读到内容，避免与 cat 抢顺序
UP_OUT=""
for _ in $(seq 1 20); do
  UP_OUT=$(api POST /terminal/shell "{\"id\":$CLIENT_ID,\"command\":\"cat /tmp/up.txt 2>/dev/null\"}")
  echo "$UP_OUT" | grep -q "$UP_TOKEN" && break
  sleep 0.5
done
echo "$UP_OUT" | grep -q "$UP_TOKEN" || fail "uploaded file not present on target: $UP_OUT"
rm -f /tmp/up.txt
log "upload ok ($UP_TOKEN round-tripped to /tmp/up.txt)"

# ---------------------------------------------------------------- 7c. 浏览器下载（B5.3）
log "check /api/file/downloadtobrowser"
DL_OUT="$TMP/dl_browser.bin"
DL_CODE=$(curl -sS -m 30 -o "$DL_OUT" -w '%{http_code}' \
  "http://127.0.0.1:$WEB_PORT/api/file/downloadtobrowser?id=$CLIENT_ID&path=/etc&target=hostname&token=$TOKEN")
[[ "$DL_CODE" == "200" ]] || fail "downloadtobrowser -> HTTP $DL_CODE ($(head -c 200 "$DL_OUT"))"
[[ -s "$DL_OUT" ]] || fail "downloadtobrowser returned empty body"
grep -q "$(hostname)" "$DL_OUT" || fail "downloaded content mismatch: $(head -c 80 "$DL_OUT")"
log "browser download ok ($(wc -c < "$DL_OUT") bytes, /etc/hostname)"

# ---------------------------------------------------------------- 7d. 大文件分块下载 + 进度（B3.6/B5.3）
log "check chunked download (>1 MiB) and progress"
api POST /terminal/shell "{\"id\":$CLIENT_ID,\"command\":\"dd if=/dev/zero of=/tmp/big.bin bs=1M count=3 2>/dev/null && wc -c < /tmp/big.bin\"}" \
  | grep -q "3145728" || fail "failed to create 3 MiB test file on target"
BIG_OUT="$TMP/big.bin"
BIG_CODE=$(curl -sS -m 120 -o "$BIG_OUT" -w '%{http_code}' \
  "http://127.0.0.1:$WEB_PORT/api/file/downloadtobrowser?id=$CLIENT_ID&path=/tmp&target=big.bin&token=$TOKEN")
[[ "$BIG_CODE" == "200" ]] || fail "big download -> HTTP $BIG_CODE"
BIG_SIZE=$(stat -c%s "$BIG_OUT")
[[ "$BIG_SIZE" == "3145728" ]] || fail "chunked download size $BIG_SIZE != 3145728"
log "chunked download ok ($BIG_SIZE bytes, $(($BIG_SIZE / 1048576 + 1)) chunks)"

api POST /file/downloadtoserver "{\"id\":$CLIENT_ID,\"path\":\"/tmp\",\"target\":\"big.bin\"}" >/dev/null
PRE=""
for _ in $(seq 1 20); do
  PRE=$(api POST /file/getdownloadper "{\"id\":$CLIENT_ID,\"target\":\"big.bin\",\"size\":3145728}")
  echo "$PRE" | grep -q '"pre":100' && break
  sleep 0.5
done
echo "$PRE" | grep -q '"pre":100' || fail "download progress stuck: $PRE"
rm -f /tmp/big.bin

# ---------------------------------------------------------------- 7e. 屏幕输入注入（B3.4）
log "check screen_input routing (no real pointer move)"
INPUT_RESP=$(api POST /terminal/shell "{\"id\":$CLIENT_ID,\"command\":\"screen_input jump x=1\"}")
echo "$INPUT_RESP" | grep -q "unknown screen_input kind" \
  || fail "screen_input not routed to the agent handler: $INPUT_RESP"
# 真实指针移动默认不跑（会打断操作者），显式开启后再验证。
if [[ "${E2E_REAL_INPUT:-0}" == "1" ]]; then
  MOVE_RESP=$(api POST /terminal/shell "{\"id\":$CLIENT_ID,\"command\":\"screen_input move x=10 y=10\"}")
  echo "$MOVE_RESP" | grep -q "input ok" || fail "real input injection failed: $MOVE_RESP"
  log "real input injection ok (mouse moved to 10,10)"
fi

# ---------------------------------------------------------------- 7f. 截图（B3.4）
log "check /api/screenshot/get"
# 截图依赖目标机的桌面环境（X11 / Wayland + grim|spectacle|import）。CI 或
# 无人值守环境常常没有这些，因此"agent 明确报告缺工具"视为 SKIP，不算失败；
# 真正的失败是面板返回非预期结构或非 PNG 数据。
SHOT="$TMP/shot.b64"
SHOT_CODE=$(curl -sS -m 60 -o "$SHOT" -w '%{http_code}' \
  "http://127.0.0.1:$WEB_PORT/api/screenshot/get?id=$CLIENT_ID&quality=normal&token=$TOKEN")
[[ "$SHOT_CODE" == "200" ]] || fail "screenshot -> HTTP $SHOT_CODE"
if head -c 400 "$SHOT" | grep -q "screenshot failed"; then
  log "skip screenshot: $(head -c 160 "$SHOT" | tr -d '\n')"
else
  python3 - "$SHOT" <<'PYEOF' || fail "screenshot payload is not a PNG"
import base64, sys
raw = open(sys.argv[1]).read().strip()
if not raw:
    raise SystemExit(1)
data = base64.b64decode(raw)
if not data.startswith(b"\x89PNG"):
    raise SystemExit(2)
PYEOF
  log "screenshot ok ($(wc -c < "$SHOT") base64 bytes, PNG verified)"
fi

# ---------------------------------------------------------------- 7g. 插件执行（B3.5）
log "check /api/runner/runplugin"
# 用一个真实的 ELF（/bin/echo）当插件：agent 落盘后执行并回显输出。
cp /bin/echo "$TMP/echo.x64.elf"
api POST /runner/upload "" >/dev/null 2>&1 || true
# runner/upload 走 multipart；这里直接用面板接口上传到服务器 plugins/
mkdir -p "$TMP/plugins" 2>/dev/null || true
PLUG_RESP=$(curl -sS -m 30 -X POST "http://127.0.0.1:$WEB_PORT/api/runner/upload" \
  -H "Token: $TOKEN" -F "file=@$TMP/echo.x64.elf")
echo "$PLUG_RESP" | grep -q '"code":0' || fail "plugin upload failed: $PLUG_RESP"

RUN_RESP=$(api POST /runner/runplugin "{\"id\":$CLIENT_ID,\"pluginName\":\"echo.x64.elf\",\"procArg\":\"plugin-ok\"}")
echo "$RUN_RESP" | grep -q "plugin-ok" || fail "plugin execution failed: $RUN_RESP"
log "plugin executed on target: $(echo "$RUN_RESP" | head -c 120)"

# ---------------------------------------------------------------- 7h. 隧道数据面（M4 端口转发）
log "check tunnel data plane (port forward through the agent)"
TUN_PORT=${TUN_PORT:-$((24000 + RANDOM % 1000))}
TGT_PORT=${TGT_PORT:-$((24500 + RANDOM % 1000))}
python3 -m http.server "$TGT_PORT" --bind 127.0.0.1 > "$TMP/http_target.log" 2>&1 &
TGT_PID=$!
sleep 0.6

api POST /tunnel/add "{\"clientId\":$CLIENT_ID,\"Port\":$TUN_PORT,\"Mode\":\"tcp\",\"Target\":\"127.0.0.1:$TGT_PORT\",\"Remark\":\"e2e-tunnel\"}" >/dev/null
TLIST=$(api POST /tunnel/list '{"page":1,"pageSize":50}')
TUN_ID=$(jget "$TLIST" 'max(i["Id"] for i in d["result"]["items"])')
api POST /tunnel/start "{\"id\":$TUN_ID,\"serverAddr\":\"127.0.0.1:$TUN_PORT\"}" | grep -q '"code":0' \
  || fail "tunnel start failed"

# 公网侧访问隧道端口，应当拿到目标 HTTP 服务的目录列表。
TUN_BODY=""
for _ in $(seq 1 40); do
  TUN_BODY=$(curl -sS -m 5 "http://127.0.0.1:$TUN_PORT/" 2>/dev/null || true)
  echo "$TUN_BODY" | grep -qi "directory listing" && break
  sleep 0.5
done
echo "$TUN_BODY" | grep -qi "directory listing" || fail "tunnel did not forward HTTP: $(echo "$TUN_BODY" | head -c 120)"
log "tunnel ok: :$TUN_PORT -> 127.0.0.1:$TGT_PORT ($(echo "$TUN_BODY" | wc -c) bytes)"

api POST /tunnel/stop "{\"id\":$TUN_ID}" >/dev/null
kill "$TGT_PID" 2>/dev/null

# ---------------------------------------------------------------- 7i. 目录列表（B5.2 配套）
log "check /api/file/ls (JSON filelist)"
LS_RESP=$(api POST /file/ls "{\"id\":$CLIENT_ID,\"path\":\"/tmp\",\"field\":\"name\",\"order\":\"ascend\"}")
echo "$LS_RESP" | grep -q '\"name\"' || fail "ls returned no entries: $LS_RESP"
echo "$LS_RESP" | grep -q 'up.txt\|big.bin\|\.ss_' || log "ls ok (no known file in /tmp, but entries present)"
LS_TOTAL=$(echo "$LS_RESP" | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["total"])' 2>/dev/null || echo 0)
[[ "$LS_TOTAL" -gt 0 ]] || fail "ls total is 0: $LS_RESP"
log "ls ok: $LS_TOTAL entries"

# ---------------------------------------------------------------- 7j. 新版 Vue3 面板托管（M6）
log "check /app/ (Vue3 panel)"
if [[ -f "$ROOT/web/dist/index.html" ]]; then
  APP_CODE=$(curl -sS -m 10 -o "$TMP/app.html" -w '%{http_code}' "http://127.0.0.1:$WEB_PORT/app/")
  [[ "$APP_CODE" == "200" ]] || fail "/app/ -> HTTP $APP_CODE"
  grep -q '<div id="app">' "$TMP/app.html" || fail "/app/ did not return the Vue shell"
  # 新增页面（截图）是懒加载 chunk，按文件名检查（不在入口 bundle 里）
  ls "$TMP"/web/dist/assets/ 2>/dev/null | grep -q '^ScreenshotView-.*\.js$' \
    || log "note: ScreenshotView chunk missing (stale build?)"
  ASSET=$(grep -oE '/app/assets/[A-Za-z0-9_.-]+\.js' "$TMP/app.html" | head -1)
  [[ -n "$ASSET" ]] || fail "no asset referenced by /app/index.html"
  ASSET_CODE=$(curl -sS -m 10 -o /dev/null -w '%{http_code}' "http://127.0.0.1:$WEB_PORT$ASSET")
  [[ "$ASSET_CODE" == "200" ]] || fail "asset $ASSET -> HTTP $ASSET_CODE"
  log "vue panel served from /app/ (asset $ASSET ok)"
else
  log "skip /app/: web/dist 未构建（cd web && pnpm build）"
fi

# ---------------------------------------------------------------- 7k. 文件读写（cat/edit → JSON 命令）
log "check /api/file/cat + /api/file/edit"
printf 'hello-from-panel\n' > /tmp/vs_cat_test.txt
CAT_RESP=$(api POST /file/cat "{\"id\":$CLIENT_ID,\"path\":\"/tmp\",\"target\":\"vs_cat_test.txt\"}")
echo "$CAT_RESP" | grep -q "hello-from-panel" || fail "cat failed: $CAT_RESP"
EDIT_RESP=$(api POST /file/edit "{\"id\":$CLIENT_ID,\"path\":\"/tmp\",\"target\":\"vs_cat_test.txt\",\"content\":\"edited-by-panel\"}")
echo "$EDIT_RESP" | grep -q '"code":0' || fail "edit failed: $EDIT_RESP"
# agent 是轮询式（约 1s 一次 + 抖动），这里等它把 fileedit 执行完。
for _ in $(seq 1 20); do
  grep -q "edited-by-panel" /tmp/vs_cat_test.txt && break
  sleep 0.5
done
grep -q "edited-by-panel" /tmp/vs_cat_test.txt || fail "edit did not reach the target file"
rm -f /tmp/vs_cat_test.txt
log "file cat/edit ok"

# ---------------------------------------------------------------- 7l. 原生 MCP（stdio）
log "check MCP stdio tools"
# MCP 必须与服务端读同一份 conf/setting.conf，因此在 $TMP（服务端 cwd）里执行。
MCP_OUT=$(printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_clients","arguments":{}}}' \
  | ( cd "$TMP" && ./vshell-server mcp 2>/dev/null ))
echo "$MCP_OUT" | grep -q '"protocolVersion"' || fail "MCP initialize failed: $(echo "$MCP_OUT" | head -c 200)"
echo "$MCP_OUT" | grep -q "$VKEY" || fail "MCP list_clients did not see the agent: $(echo "$MCP_OUT" | head -c 300)"

# 资源：resources/list + resources/read（STATUS.md）
MCP_RES=$(printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"resources/list"}' \
  '{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"vshell://docs/STATUS.md"}}' \
  | ( cd "$TMP" && ./vshell-server mcp 2>/dev/null ))
echo "$MCP_RES" | grep -q 'vshell://docs/ROADMAP.md' || fail "resources/list missing ROADMAP: $(echo "$MCP_RES" | head -c 200)"
echo "$MCP_RES" | grep -q '还原状态' || fail "resources/read did not return STATUS.md"
log "mcp ok: initialize + list_clients（含 agent vkey）+ resources（list/read）"

# ---------------------------------------------------------------- 7m. wget / OSS 暂存（B5.2 配套）
log "check /api/file/wget + /api/file/downloadtooss"
WGET_URL="http://127.0.0.1:$WEB_PORT/health"
WGET_RESP=$(api POST /file/wget "{\"id\":$CLIENT_ID,\"path\":\"/tmp\",\"target\":\"$WGET_URL\"}")
echo "$WGET_RESP" | grep -q '"code":0' || fail "wget failed: $WGET_RESP"
for _ in $(seq 1 20); do
  [[ -s /tmp/health ]] && break
  sleep 0.5
done
[[ -s /tmp/health ]] || fail "wget did not create /tmp/health on target"
rm -f /tmp/health
log "wget ok (health 页已落到目标 /tmp)"

OSS_RESP=$(api POST /file/downloadtooss "{\"id\":$CLIENT_ID,\"path\":\"/etc\",\"target\":\"hostname\"}")
echo "$OSS_RESP" | grep -q "oss upload not implemented" \
  || fail "downloadtooss should report unimplemented OSS: $OSS_RESP"
ls "$TMP"/downloads/*/hostname >/dev/null 2>&1 || fail "downloadtooss did not stage the file (TMP=$TMP)"
log "oss staging ok (文件已暂存，OSS 上传明确未实现)"

# ---------------------------------------------------------------- 7n. 文件操作：touch/mv/rm/disk
log "check file touch/mv/rm/disk"
api POST /file/touch "{\"id\":$CLIENT_ID,\"path\":\"/tmp\",\"target\":\"vs_touch.txt\"}" | grep -q '"code":0' \
  || fail "touch failed"
for _ in $(seq 1 20); do [[ -f /tmp/vs_touch.txt ]] && break; sleep 0.5; done
[[ -f /tmp/vs_touch.txt ]] || fail "touch did not create the file"

api POST /file/modifytime "{\"id\":$CLIENT_ID,\"path\":\"/tmp\",\"target\":\"vs_touch.txt\",\"time\":\"2020-01-02 03:04:05\"}" >/dev/null
# agent 是轮询式（约 1s 一次 + 抖动）：轮询等它把 filetouch+time 执行完
YEAR=""
for _ in $(seq 1 20); do
  YEAR=$(stat -c %y /tmp/vs_touch.txt 2>/dev/null | cut -d- -f1)
  [[ "$YEAR" == "2020" ]] && break
  sleep 0.5
done
[[ "$YEAR" == "2020" ]] || fail "modifytime did not apply (year=$YEAR)"

api POST /file/mv "{\"id\":$CLIENT_ID,\"path\":\"/tmp/vs_touch.txt|<-/tmp/vs_moved.txt\"}" | grep -q '"code":0' \
  || fail "mv failed"
for _ in $(seq 1 20); do [[ -f /tmp/vs_moved.txt ]] && break; sleep 0.5; done
[[ -f /tmp/vs_moved.txt ]] || fail "mv did not rename the file"

api POST /file/rm "{\"id\":$CLIENT_ID,\"path\":\"/tmp/vs_moved.txt\"}" | grep -q '"code":0' || fail "rm failed"
for _ in $(seq 1 20); do [[ ! -f /tmp/vs_moved.txt ]] && break; sleep 0.5; done
[[ ! -f /tmp/vs_moved.txt ]] || fail "rm did not delete the file"

DISK=$(api POST /file/getdisk "{\"id\":$CLIENT_ID}")
echo "$DISK" | grep -q '"data"' || fail "getdisk failed: $DISK"
log "file ops ok: touch → modifytime → mv → rm → disk"

# ---------------------------------------------------------------- 7o. 进程结束 / 持久化命令（JSON 化）
log "check delprocess + install/remove (JSON commands)"
# 起一个 sleep 进程，通过 delprocess 杀掉（agent 侧走 kill JSON 命令）
sleep 300 &
VICTIM=$!
sleep 0.2
api POST /client/delprocess "{\"id\":$CLIENT_ID,\"pid\":$VICTIM,\"name\":\"\"}" | grep -q '"code":0' \
  || fail "delprocess failed"
for _ in $(seq 1 20); do kill -0 "$VICTIM" 2>/dev/null || break; sleep 0.5; done
kill -0 "$VICTIM" 2>/dev/null && fail "delprocess did not kill pid $VICTIM"
log "delprocess ok (pid $VICTIM 已被目标机 kill)"

# install/remove 走 persist/cleanup 命令：只断言接口成功与命令可达，不做真实持久化
api POST /install/install "{\"id\":$CLIENT_ID,\"name\":\"e2e-persist\"}" | grep -q '"code":0' \
  || fail "install failed"
api POST /install/remove "{\"id\":$CLIENT_ID,\"name\":\"e2e-persist\"}" | grep -q '"code":0' \
  || fail "remove failed"
log "install/remove ok (persist/cleanup 命令已下发)"

# ---------------------------------------------------------------- 7p. 官方协议服务端 + agent 模拟器（R27）
log "check official-tcp server + official agent simulator"
OTCP_PORT=$((25000 + RANDOM % 500))
"$TMP/vshell-server" official-tcp -listen "127.0.0.1:$OTCP_PORT" -vkey OFFICIAL-VK -cmd id \
  > "$TMP/otcp.log" 2>&1 &
OTCP_PID=$!
sleep 1
SIM_OUT=$("$TMP/vshell-server" official-agent-sim -addr "127.0.0.1:$OTCP_PORT" -vkey OFFICIAL-VK -timeout 6 2>/dev/null || true)
echo "$SIM_OUT" | grep -q "registered=true tasks=1" || fail "official agent sim failed: $SIM_OUT"
grep -q "main channel registered" "$TMP/otcp.log" || fail "official-tcp server did not register main channel"
grep -q "uid=" "$TMP/otcp.log" || fail "official-tcp server did not receive command output"
kill "$OTCP_PID" 2>/dev/null
log "official-tcp ok: 模拟 agent 完成认证/通道注册/任务执行"

# ---------------------------------------------------------------- 7q. 面板端点覆盖补全（R38）
log "check remaining panel endpoints"
# --- 认证类
api GET /getUserInfo | grep -q '"username"' || fail "getUserInfo failed"
api GET /getMenuList | grep -q '"code":0' || fail "getMenuList failed"
api GET /getPermCode | grep -q '"code":0' || fail "getPermCode failed"
api GET /logout | grep -q '"code":0' || fail "logout failed"

# --- 概览 / 插件列表 / 设置
api GET /dashboard/info | grep -q '"version"' || fail "dashboard/info failed"
api GET /runner/list | grep -q '"code":0' || fail "runner/list failed"
api POST /setting/edit "{\"wx_key\":\"e2e-wx\",\"dingding_access_token\":\"e2e-dt\",\"dingding_key_word\":\"e2e-kw\"}" \
  | grep -q '"code":0' || fail "setting/edit failed"
api GET /setting/get | grep -q 'e2e-wx' || fail "setting/get did not persist"

# --- 客户端列表过滤（status / search / 排序）
ONLINE_CNT=$(api POST /client/list '{"page":1,"pageSize":50,"status":1}' | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['result']['total'])")
[[ "$ONLINE_CNT" -ge 1 ]] || fail "client/list status=1 未返回在线客户端（NowConn/Status 判定不一致？）"
SEARCH_CNT=$(api POST /client/list "{\"page\":1,\"pageSize\":50,\"search\":\"$(hostname)\"}" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['result']['total'])")
[[ "$SEARCH_CNT" -ge 1 ]] || fail "client/list search 未按主机名命中"
SORTED_HEAD=$(api POST /client/list '{"page":1,"pageSize":50,"field":"Id","order":"descend"}' | python3 -c "import sys,json;d=json.load(sys.stdin);print(d['result']['items'][0]['Id'])")
[[ -n "$SORTED_HEAD" ]] || fail "client/list 排序返回空"

# --- 客户端备注 / 删除文件
api POST /client/editremark "{\"id\":$CLIENT_ID,\"remark\":\"e2e-note\"}" | grep -q '"code":0' \
  || fail "client/editremark failed"
api POST /client/list '{"page":1,"pageSize":50}' | grep -q 'e2e-note' || fail "client remark not saved"
printf 'del-me\n' > /tmp/vs_delme.txt
api POST /client/delfile "{\"id\":$CLIENT_ID,\"paths\":\"/tmp/vs_delme.txt\"}" | grep -q '"code":0' \
  || fail "client/delfile failed"
for _ in $(seq 1 20); do [[ ! -f /tmp/vs_delme.txt ]] && break; sleep 0.5; done
[[ ! -f /tmp/vs_delme.txt ]] || fail "client/delfile did not remove the file"

# --- 文件 mkdir / rmlist
api POST /file/mkdir "{\"id\":$CLIENT_ID,\"path\":\"/tmp/vs_e2e_dir\"}" | grep -q '"code":0' \
  || fail "file/mkdir failed"
for _ in $(seq 1 20); do [[ -d /tmp/vs_e2e_dir ]] && break; sleep 0.5; done
[[ -d /tmp/vs_e2e_dir ]] || fail "file/mkdir did not create the dir"
touch /tmp/vs_e2e_dir/a.txt /tmp/vs_e2e_dir/b.txt
api POST /file/rmlist "{\"id\":$CLIENT_ID,\"path\":\"/tmp/vs_e2e_dir\",\"name\":\"a.txt|<-b.txt\"}" \
  | grep -q '"code":0' || fail "file/rmlist failed"
for _ in $(seq 1 20); do [[ ! -f /tmp/vs_e2e_dir/a.txt && ! -f /tmp/vs_e2e_dir/b.txt ]] && break; sleep 0.5; done
[[ ! -f /tmp/vs_e2e_dir/a.txt && ! -f /tmp/vs_e2e_dir/b.txt ]] || fail "file/rmlist did not delete files"
rmdir /tmp/vs_e2e_dir 2>/dev/null || true

# --- 监听器 edit / editremark / stop / del / dellist（用临时监听器，不影响主链路）
TMP_LPORT=$((26000 + RANDOM % 500))
api POST /listener/add "{\"Mode\":\"http\",\"Vkey\":\"E2E-TMP\",\"Host\":\"127.0.0.1\",\"Port\":$TMP_LPORT,\"Remark\":\"tmp\"}" >/dev/null
TMP_LID=$(api POST /listener/list '{"page":1,"pageSize":50}' | python3 -c '
import sys,json
d=json.load(sys.stdin)
print([i["Id"] for i in d["result"]["items"] if i["Vkey"]=="E2E-TMP"][0])')
api POST /listener/edit "{\"Id\":$TMP_LID,\"Mode\":\"http\",\"Vkey\":\"E2E-TMP2\",\"Remark\":\"tmp2\",\"OssUrl\":\"\",\"vip\":false}" \
  | grep -q '"code":0' || fail "listener/edit failed"
api POST /listener/editremark "{\"id\":$TMP_LID,\"remark\":\"tmp-remark\"}" | grep -q '"code":0' \
  || fail "listener/editremark failed"
api POST /listener/stop "{\"id\":$TMP_LID}" | grep -q '"code":0' || fail "listener/stop failed"
api POST /listener/del "{\"id\":$TMP_LID}" | grep -q '"code":0' || fail "listener/del failed"

api POST /listener/add "{\"Mode\":\"http\",\"Vkey\":\"E2E-TMP3\",\"Host\":\"127.0.0.1\",\"Port\":$((TMP_LPORT+1)),\"Remark\":\"tmp\"}" >/dev/null
TMP_LID2=$(api POST /listener/list '{"page":1,"pageSize":50}' | python3 -c '
import sys,json
d=json.load(sys.stdin)
print([i["Id"] for i in d["result"]["items"] if i["Vkey"]=="E2E-TMP3"][0])')
api POST /listener/dellist "{\"id\":[$TMP_LID2]}" | grep -q '"code":0' || fail "listener/dellist failed"

# --- 隧道 edit / editremark / dellist / del
api POST /tunnel/add "{\"clientId\":$CLIENT_ID,\"Port\":$((TMP_LPORT+10)),\"Mode\":\"tcp\",\"Target\":\"127.0.0.1:22\",\"Remark\":\"vtmp\"}" >/dev/null
TMP_TID=$(api POST /tunnel/list '{"page":1,"pageSize":50}' | python3 -c "
import sys,json
d=json.load(sys.stdin)
items=d['result']['items'] or []
ids=[i['Id'] for i in items if i.get('Port')==$((TMP_LPORT+10))]
print(ids[0] if ids else '')")
[[ -n "$TMP_TID" ]] || fail "temp tunnel not created (list=$(api POST /tunnel/list '{"page":1,"pageSize":50}' | head -c 200))"
api POST /tunnel/edit "{\"Id\":$TMP_TID,\"Port\":$((TMP_LPORT+11)),\"Mode\":\"tcp\",\"Target\":\"127.0.0.1:23\",\"Remark\":\"vtmp2\"}" \
  | grep -q '"code":0' || fail "tunnel/edit failed"
api POST /tunnel/editremark "{\"id\":$TMP_TID,\"remark\":\"vtmp3\"}" | grep -q '"code":0' \
  || fail "tunnel/editremark failed"
api POST /tunnel/dellist "{\"id\":[$TMP_TID]}" | grep -q '"code":0' || fail "tunnel/dellist failed"

# tunnel/del（单条删除）
api POST /tunnel/add "{\"clientId\":$CLIENT_ID,\"Port\":$((TMP_LPORT+12)),\"Mode\":\"tcp\",\"Target\":\"127.0.0.1:24\",\"Remark\":\"vtmp4\"}" >/dev/null
TMP_TID2=$(api POST /tunnel/list '{"page":1,"pageSize":50}' | python3 -c "
import sys,json
d=json.load(sys.stdin)
items=d['result']['items'] or []
ids=[i['Id'] for i in items if i.get('Port')==$((TMP_LPORT+12))]
print(ids[0] if ids else '')")
[[ -n "$TMP_TID2" ]] || fail "temp tunnel2 not created"
api POST /tunnel/del "{\"id\":$TMP_TID2}" | grep -q '"code":0' || fail "tunnel/del failed"
log "remaining panel endpoints ok (auth/dashboard/runner/setting/client/file/listener/tunnel)"

# ---------------------------------------------------------------- 8. 面板契约
log "check /client/list field contract (B0.8)"
LIST=$(api POST /client/list '{"page":1,"pageSize":10}')
for key in Id OsName UserName HostName LocalIP Addr Tp IsConnect Remark; do
  jget "$LIST" "(1 if '$key' in d['result']['items'][0] else (_ for _ in ()).throw(SystemExit('$key')))" \
    >/dev/null 2>&1 || fail "client/list missing key: $key"
done
jget "$LIST" 'd["result"]["items"][0]["IsConnect"]' | grep -qx True || fail "IsConnect should be true"

# ---------------------------------------------------------------- 9. 下载端点（GET）
# 原版从磁盘取模板（stageless/<tp>_<arch>），这里放一个假模板让下载链路可验证。
mkdir -p "$TMP/stageless" && printf 'MZ-e2e-template' > "$TMP/stageless/http_windows_amd64"

log "check GET /download/stage (B0.7)"
DL="$TMP/stage.bin"
CODE=$(curl -sS -m 20 -o "$DL" -w '%{http_code}' \
  "http://127.0.0.1:$WEB_PORT/api/download/stage?arch=windows_amd64&id=$LST_ID&token=$TOKEN")
[[ "$CODE" == "200" ]] || fail "GET /download/stage -> HTTP $CODE"
[[ -s "$DL" ]] || fail "GET /download/stage returned empty body"
log "download ok ($(wc -c < "$DL") bytes)"

# ---------------------------------------------------------------- 10. 批量删除（JSON 数组）
log "check delList with JSON array (B0.9)"
api POST /client/add '{"ip":"203.0.113.9:4444"}' >/dev/null
LIST=$(api POST /client/list '{"page":1,"pageSize":50}')
DUP=$(jget "$LIST" 'max(i["Id"] for i in d["result"]["items"])')
RESP=$(api POST /client/dellist "{\"id\":[$DUP]}")
echo "$RESP" | grep -q '"code":0' || fail "dellist failed: $RESP"
LIST=$(api POST /client/list '{"page":1,"pageSize":50}')
jget "$LIST" "[i['Id'] for i in d['result']['items']]" | grep -qw "$DUP" \
  && fail "dellist did not remove $DUP"

# ---------------------------------------------------------------- 11. KCP 通道
log "check KCP channel (B0.6)"
KCP_PORT=${KCP_PORT:-$((20000 + RANDOM % 1000))}
KCP_VKEY="vk_kcp_$$"
api POST /listener/add "{\"Mode\":\"kcp\",\"Vkey\":\"$KCP_VKEY\",\"Remark\":\"e2e-kcp\",\"Host\":\"127.0.0.1\",\"Port\":$KCP_PORT,\"PingInterval\":1}" >/dev/null
LST=$(api POST /listener/list '{"page":1,"pageSize":50}')
KCP_ID=$(jget "$LST" 'max(i["Id"] for i in d["result"]["items"])')
KCP_SALT=$(jget "$LST" "next(i['EncryptSalt'] for i in d['result']['items'] if i['Id']==$KCP_ID)")
log "kcp listener id=$KCP_ID salt=${KCP_SALT:-<empty>}"

( cd "$ROOT/agent" && go build -ldflags "\
  -X main.ServerAddr=kcp://127.0.0.1:$KCP_PORT \
  -X main.VerifyKey=$KCP_VKEY \
  -X main.EncryptSalt=$KCP_SALT" -o "$TMP/agent_kcp" . ) || fail "kcp agent build"
"$TMP/agent_kcp" > "$TMP/agent_kcp.log" 2>&1 &
AGENT_KCP_PID=$!
KCP_CLIENT=0
for _ in $(seq 1 60); do
  LIST=$(api POST /client/list '{"page":1,"pageSize":50}')
  KCP_CLIENT=$(jget "$LIST" "next((i['Id'] for i in d['result']['items'] if i['Tp']=='kcp' and i['IsConnect']), 0)" 2>/dev/null || echo 0)
  [[ "$KCP_CLIENT" != "0" ]] && break
  sleep 0.5
done
[[ "$KCP_CLIENT" != "0" ]] || { tail -5 "$TMP/agent_kcp.log"; fail "kcp agent never came online"; }
log "kcp client $KCP_CLIENT online"

OUT=$(api POST /terminal/shell "{\"id\":$KCP_CLIENT,\"command\":\"id\"}")
echo "$OUT" | grep -q '"code":0' || fail "kcp terminal/shell failed: $OUT"
echo "$OUT" | grep -qE 'uid=[0-9]+' || fail "unexpected kcp output: $OUT"
kill "$AGENT_KCP_PID" 2>/dev/null

# ---------------------------------------------------------------- 12. WebSocket 通道
log "check WebSocket channel (B1.5)"
WS_PORT=${WS_PORT:-$((21000 + RANDOM % 1000))}
WS_VKEY="vk_ws_$$"
api POST /listener/add "{\"Mode\":\"websocket\",\"Vkey\":\"$WS_VKEY\",\"Remark\":\"e2e-ws\",\"Host\":\"127.0.0.1\",\"Port\":$WS_PORT,\"PingInterval\":1}" >/dev/null
LST=$(api POST /listener/list '{"page":1,"pageSize":50}')
WS_ID=$(jget "$LST" 'max(i["Id"] for i in d["result"]["items"])')
WS_SALT=$(jget "$LST" "next(i['EncryptSalt'] for i in d['result']['items'] if i['Id']==$WS_ID)")
log "ws listener id=$WS_ID salt=${WS_SALT:-<empty>}"

( cd "$ROOT/agent" && go build -ldflags "\
  -X main.ServerAddr=ws://127.0.0.1:$WS_PORT \
  -X main.VerifyKey=$WS_VKEY \
  -X main.EncryptSalt=$WS_SALT" -o "$TMP/agent_ws" . ) || fail "ws agent build"
"$TMP/agent_ws" > "$TMP/agent_ws.log" 2>&1 &
AGENT_WS_PID=$!
WS_CLIENT=0
for _ in $(seq 1 60); do
  LIST=$(api POST /client/list '{"page":1,"pageSize":50}')
  WS_CLIENT=$(jget "$LIST" "next((i['Id'] for i in d['result']['items'] if i['Tp']=='websocket' and i['IsConnect']), 0)" 2>/dev/null || echo 0)
  [[ "$WS_CLIENT" != "0" ]] && break
  sleep 0.5
done
[[ "$WS_CLIENT" != "0" ]] || { tail -5 "$TMP/agent_ws.log"; fail "ws agent never came online"; }
log "ws client $WS_CLIENT online"

OUT=$(api POST /terminal/shell "{\"id\":$WS_CLIENT,\"command\":\"id\"}")
echo "$OUT" | grep -qE 'uid=[0-9]+' || fail "ws terminal/shell failed: $OUT"
kill "$AGENT_WS_PID" 2>/dev/null

# ---------------------------------------------------------------- 13. DNS 通道
log "check DNS channel (B1.4)"
DNS_PORT=${DNS_PORT:-$((22000 + RANDOM % 1000))}
DNS_VKEY="vk_dns_$$"
DNS_ZONE="c2.e2e.test"
api POST /listener/add "{\"Mode\":\"dns\",\"Vkey\":\"$DNS_VKEY\",\"Remark\":\"e2e-dns\",\"Host\":\"127.0.0.1\",\"Port\":$DNS_PORT,\"DNSDomain\":\"$DNS_ZONE\",\"MaxDNSsize\":512,\"PingInterval\":1}" >/dev/null
LST=$(api POST /listener/list '{"page":1,"pageSize":50}')
DNS_ID=$(jget "$LST" 'max(i["Id"] for i in d["result"]["items"])')
log "dns listener id=$DNS_ID zone=$DNS_ZONE udp=127.0.0.1:$DNS_PORT"

( cd "$ROOT/agent" && go build -ldflags "\
  -X main.ServerAddr=dns://$DNS_ZONE \
  -X main.VerifyKey=$DNS_VKEY \
  -X main.DNSServer=127.0.0.1:$DNS_PORT" -o "$TMP/agent_dns" . ) || fail "dns agent build"
"$TMP/agent_dns" > "$TMP/agent_dns.log" 2>&1 &
AGENT_DNS_PID=$!
DNS_CLIENT=0
for _ in $(seq 1 60); do
  LIST=$(api POST /client/list '{"page":1,"pageSize":50}')
  DNS_CLIENT=$(jget "$LIST" "next((i['Id'] for i in d['result']['items'] if i['Tp']=='dns' and i['IsConnect']), 0)" 2>/dev/null || echo 0)
  [[ "$DNS_CLIENT" != "0" ]] && break
  sleep 0.5
done
[[ "$DNS_CLIENT" != "0" ]] || { tail -20 "$TMP/agent_dns.log"; fail "dns agent never came online"; }
log "dns client $DNS_CLIENT online"

# DNS 通道是"一轮一问"，首次任务可能落在下一轮轮询（~1-2s），
# 给这条命令单独的更长超时。
OUT=$(curl -sS -m 40 -X POST "http://127.0.0.1:$WEB_PORT/api/terminal/shell" \
  -H "Token: $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"id\":$DNS_CLIENT,\"command\":\"id\"}")
echo "$OUT" | grep -qE 'uid=[0-9]+' || { echo "--- agent_dns.log ---"; tail -30 "$TMP/agent_dns.log"; fail "dns terminal/shell failed: $OUT"; }
kill "$AGENT_DNS_PID" 2>/dev/null

# ---------------------------------------------------------------- 14. 面板生成载荷 → 直接上线（B2.2）
log "check panel-generated payload (no ldflags)"
PAY_PORT=${PAY_PORT:-$((23000 + RANDOM % 1000))}
PAY_VKEY="vk_pay_$$"
api POST /listener/add "{\"Mode\":\"http\",\"Vkey\":\"$PAY_VKEY\",\"Remark\":\"e2e-payload\",\"Host\":\"127.0.0.1\",\"Port\":$PAY_PORT,\"PingInterval\":1}" >/dev/null
LST=$(api POST /listener/list '{"page":1,"pageSize":50}')
PAY_ID=$(jget "$LST" 'max(i["Id"] for i in d["result"]["items"])')

# 模板：不带任何 ldflags 编译一个 agent，按原版命名放到 stageless/<tp>_<arch>
( cd "$ROOT/agent" && go build -o "$TMP/stageless/http_linux_amd64" . ) || fail "plain agent build"
PAYLOAD="$TMP/payload_linux_amd64"
CODE=$(curl -sS -m 30 -o "$PAYLOAD" -w '%{http_code}' \
  "http://127.0.0.1:$WEB_PORT/api/download/stageless?id=$PAY_ID&arch=linux_amd64&token=$TOKEN")
[[ "$CODE" == "200" ]] || fail "download stageless -> HTTP $CODE"
TPL_SIZE=$(stat -c%s "$TMP/stageless/http_linux_amd64")
PAY_SIZE=$(stat -c%s "$PAYLOAD")
[[ "$PAY_SIZE" -gt "$TPL_SIZE" ]] || fail "payload not larger than template ($PAY_SIZE <= $TPL_SIZE)"
log "payload built: $PAY_SIZE bytes (template $TPL_SIZE + config block)"

chmod +x "$PAYLOAD"
"$PAYLOAD" > "$TMP/agent_payload.log" 2>&1 &
AGENT_PAY_PID=$!
PAY_CLIENT=0
for _ in $(seq 1 60); do
  LIST=$(api POST /client/list '{"page":1,"pageSize":50}')
  PAY_CLIENT=$(jget "$LIST" "next((i['Id'] for i in d['result']['items'] if i['VerifyKey']=='$PAY_VKEY' and i['IsConnect']), 0)" 2>/dev/null || echo 0)
  [[ "$PAY_CLIENT" != "0" ]] && break
  sleep 0.5
done
[[ "$PAY_CLIENT" != "0" ]] || { tail -10 "$TMP/agent_payload.log"; fail "panel payload never came online"; }
log "panel payload client $PAY_CLIENT online (config read from file trailer)"

OUT=$(api POST /terminal/shell "{\"id\":$PAY_CLIENT,\"command\":\"id\"}")
echo "$OUT" | grep -qE 'uid=[0-9]+' || fail "panel payload terminal/shell failed: $OUT"
kill "$AGENT_PAY_PID" 2>/dev/null

log "PASS: HTTP + KCP + WebSocket + DNS round trip, panel contract, download, delList, panel-generated payload"
exit 0
