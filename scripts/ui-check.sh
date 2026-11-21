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
expect "$TMP/files.html" '重命名' '文件页缺少重命名按钮'
expect "$TMP/files.html" '下载' '文件页缺少下载按钮'
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

log "渲染 #/listeners"
render '#/listeners' "$TMP/listeners.html"
expect "$TMP/listeners.html" '监听地址' '监听器页表头缺失'
expect "$TMP/listeners.html" 'UI-VK' '监听器页没有真实 Vkey（字段契约不符？）'
expect "$TMP/listeners.html" '运行' '监听器页没有状态标记'

log "渲染 #/download"
render '#/download' "$TMP/download.html"
expect "$TMP/download.html" 'Stageless' '下载页缺少 Stageless 按钮'
expect "$TMP/download.html" '架构' '下载页缺架构选择'

log "渲染 #/tunnels"
render '#/tunnels' "$TMP/tunnels.html"
expect "$TMP/tunnels.html" '服务器端口' '隧道页表头缺失'

log "渲染 #/plugins/1"
render '#/plugins/1' "$TMP/plugins.html"
expect "$TMP/plugins.html" '服务器插件' '插件页卡片缺失'

log "渲染 #/settings"
render '#/settings' "$TMP/settings.html"
expect "$TMP/settings.html" '企业微信' '设置页字段缺失'

log "渲染 #/screen/1"
render '#/screen/1' "$TMP/screen.html"
expect "$TMP/screen.html" '客户端 #1' '屏幕页未渲染'

log "渲染 #/screenshot/1"
render '#/screenshot/1' "$TMP/shot.html"
expect "$TMP/shot.html" '重新截图' '截图页按钮缺失'

# ---------------- 点击级流程（CDP：真填表单 / 真点按钮） ----------------
log "点击级：登录表单 → 仪表盘"
LOGIN_JS=$(cat <<'JS'
(async () => {
  const inputs = [...document.querySelectorAll('input')];
  if (inputs.length < 2) return 'NO_INPUTS';
  const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
  set(inputs[0], 'admin');
  set(inputs[1], 'p1');
  await new Promise(r => setTimeout(r, 300));
  const btn = [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录');
  if (!btn) return 'NO_BUTTON';
  btn.click();
  await new Promise(r => setTimeout(r, 2500));
  return location.hash + '|' + document.body.innerText.replace(/\n+/g, ' ');
})()
JS
)
LOGIN_OUT=$(cd "$ROOT" && go run ./tools/uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$LOGIN_JS" 2>&1)
echo "$LOGIN_OUT" | grep -q '#/dashboard' || fail "登录表单未跳转到仪表盘: $(echo "$LOGIN_OUT" | head -c 160)"
echo "$LOGIN_OUT" | grep -q '运行信息\|仪表盘' || fail "登录后页面内容异常: $(echo "$LOGIN_OUT" | head -c 160)"

log "点击级：登录 → 切到客户端页 → 点刷新"
CLICK_JS=$(cat <<'JS'
(async () => {
  // 同一个页面上下文里完成：登录 → hash 路由切换 → 点击刷新
  // （不要用 location.replace 整页跳转：会丢失内存里的登录态，且脚本随之中断）
  const inputs = [...document.querySelectorAll('input')];
  if (inputs.length < 2) return 'NO_INPUTS';
  const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
  set(inputs[0], 'admin'); set(inputs[1], 'p1');
  await new Promise(r => setTimeout(r, 300));
  [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录')?.click();
  await new Promise(r => setTimeout(r, 2200));

  location.hash = '#/clients';
  await new Promise(r => setTimeout(r, 1500));
  const btn = [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '刷新');
  if (!btn) return 'NO_REFRESH_BUTTON|' + document.body.innerText.replace(/\n+/g, ' ').slice(0, 120);
  btn.click();
  await new Promise(r => setTimeout(r, 1500));
  return document.body.innerText.replace(/\n+/g, ' ');
})()
JS
)
CLICK_OUT=$(cd "$ROOT" && go run ./tools/uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$CLICK_JS" 2>&1)
echo "$CLICK_OUT" | grep -q "$(hostname)" || fail "客户端页刷新后没有主机名: $(echo "$CLICK_OUT" | head -c 200)"

log "点击级：文件页重命名（ElMessageBox 输入框）"
# 造一个待重命名的文件：agent 与本机同机，直接本地写即可（避免等 agent 轮询）
printf 'hi\n' > /tmp/ui-rename-me.txt
RENAME_JS=$(cat <<'JS'
(async () => {
  const inputs = [...document.querySelectorAll('input')];
  if (inputs.length < 2) return 'NO_INPUTS';
  const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
  set(inputs[0], 'admin'); set(inputs[1], 'p1');
  await new Promise(r => setTimeout(r, 300));
  [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录')?.click();
  await new Promise(r => setTimeout(r, 2200));

  location.hash = '#/files/1';
  await new Promise(r => setTimeout(r, 1500));

  // 切到 /tmp（文件页默认打开根目录）
  const pathInput = [...document.querySelectorAll('input')].find(i => i.value === '/');
  if (!pathInput) return 'NO_PATH_INPUT';
  pathInput.value = '/tmp';
  pathInput.dispatchEvent(new Event('input', {bubbles: true}));
  await new Promise(r => setTimeout(r, 200));
  [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '打开')?.click();
  await new Promise(r => setTimeout(r, 2000));

  // 找到 ui-rename-me.txt 所在行，点该行的"重命名"
  const rows = [...document.querySelectorAll('.el-table__row')];
  const row = rows.find(r => r.innerText.includes('ui-rename-me.txt'));
  if (!row) return 'NO_TARGET_ROW|' + document.body.innerText.replace(/\n+/g, ' ').slice(0, 160);
  const btn = [...row.querySelectorAll('button')].find(b => b.innerText.trim() === '重命名');
  if (!btn) return 'NO_RENAME_BTN';
  btn.click();
  await new Promise(r => setTimeout(r, 600));

  // 弹框输入新名字并确定
  const box = document.querySelector('.el-message-box__input input') || document.querySelector('.el-message-box input');
  if (!box) return 'NO_DIALOG';
  box.value = 'ui-renamed.txt';
  box.dispatchEvent(new Event('input', {bubbles: true}));
  await new Promise(r => setTimeout(r, 200));
  [...document.querySelectorAll('.el-message-box button')].find(b => b.innerText.trim() === '确定')?.click();
  await new Promise(r => setTimeout(r, 2000));
  return document.body.innerText.replace(/\n+/g, ' ');
})()
JS
)
RENAME_OUT=$(cd "$ROOT" && go run ./tools/uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$RENAME_JS" 2>&1)
echo "$RENAME_OUT" | grep -q 'ui-renamed.txt' || fail "重命名未生效: $(echo "$RENAME_OUT" | head -c 200)"
[[ -f /tmp/ui-renamed.txt ]] || fail "目标机上没有 ui-renamed.txt"
rm -f /tmp/ui-renamed.txt /tmp/ui-rename-me.txt

log "全部渲染 + 点击级流程通过（login→dashboard，clients 刷新，files 重命名）"
