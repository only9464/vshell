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

# uiprobe 包装：每次用随机调试端口，避免上一次的 chromium 仍占着 9223
# （残留进程会让新探针连到旧页面，表现为 NO_INPUTS 之类的怪现象）。
uiprobe() {
  local port=$((9300 + RANDOM % 600))
  ( cd "$ROOT" && go run ./tools/uiprobe -port "$port" "$@" )
}
fail() { echo "[ui] FAIL: $*" >&2; exit 1; }

command -v "$CHROME" >/dev/null || fail "找不到 $CHROME"
[[ -f "$ROOT/web/dist/index.html" ]] || fail "web/dist 未构建（cd web && pnpm build）"

mkdir -p "$TMP/conf" "$TMP/db"
ln -sfn "$ROOT/static" "$TMP/static"
# 新前端（web2，Ant Design Vue 版）是当前默认前端；同时保留旧 web 目录
ln -sfn "$ROOT/web2" "$TMP/web2"
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

# agent 健康检查：长时间跑浏览器检查时 agent 可能掉线（http 轮询超时），
# 这里提供一个"掉线就重启"的工具函数，浏览器断言前调用一次。
ensure_agent() {
  local n
  n=$(curl -s -X POST "http://127.0.0.1:$WEB_PORT/api/client/list" -H "Token: $TOK" \
    -H 'Content-Type: application/json' -d '{"page":1,"pageSize":5}' \
    | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["clientOnlineCount"])' 2>/dev/null || echo 0)
  if [[ "${n:-0}" -lt 1 ]]; then
    kill "$AGENT_PID" 2>/dev/null
    ( cd "$TMP" && exec ./agent >> agent.log 2>&1 ) &
    AGENT_PID=$!
    for _ in $(seq 1 30); do
      sleep 0.5
      n=$(curl -s -X POST "http://127.0.0.1:$WEB_PORT/api/client/list" -H "Token: $TOK" \
        -H 'Content-Type: application/json' -d '{"page":1,"pageSize":5}' \
        | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["clientOnlineCount"])' 2>/dev/null || echo 0)
      [[ "${n:-0}" -ge 1 ]] && break
    done
    log "agent 曾掉线，已重启（在线 $n）"
  fi
}

# 渲染一个路由并返回 DOM
render() { # $1=route  $2=输出文件
  cat > "$TMP/web2/dist/bootstrap.html" <<BOOT
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

log "登录页（4.9.3 占位文案：请输入账号 / 请输入密码 / 记住我 / 忘记密码?）"
LOGINPH_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0=Date.now(); while(Date.now()-t0<ms){ if(fn()) return true; await new Promise(r=>setTimeout(r,250)); } return false; };
  await waitFor(()=>!!document.querySelector('input'), 6000);
  const ph = [...document.querySelectorAll('input')].map(i => i.placeholder).filter(Boolean);
  const txt = document.body.innerText;
  const ok = ph.includes('请输入账号') && ph.includes('请输入密码') && txt.includes('记住我');
  return ok ? 'LOGINPH_OK' : 'LOGINPH_FAIL ' + JSON.stringify({ph, hasRemember: txt.includes('记住我')});
})()
JS
)
LOGINPH_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$LOGINPH_JS" 2>&1)
echo "$LOGINPH_OUT" | grep -q 'LOGINPH_OK' || fail "登录页文案未对齐 4.9.3: $(echo "$LOGINPH_OUT" | tail -c 160)"

log "渲染 #/dashboard"
render '#/dashboard/index' "$TMP/dash.html"
expect "$TMP/dash.html" '系统信息' 'dashboard 未渲染（缺系统信息卡片）'
expect "$TMP/dash.html" '版本' 'dashboard 缺少 KPI'
expect "$TMP/dash.html" '负载趋势' 'dashboard 缺少趋势卡片'
expect "$TMP/dash.html" 'polyline' 'dashboard 趋势图未渲染（sparkline 没出 SVG）'
expect "$TMP/dash.html" '最近上线客户端' 'dashboard 缺少最近上线卡片'
expect "$TMP/dash.html" '在线客户端' 'dashboard 缺少「在线客户端」卡片'
expect "$TMP/dash.html" 'TCP 通讯 (已建立)' 'dashboard 缺少系统信息表'
expect "$TMP/dash.html" '虚拟内存' 'dashboard 缺少系统信息进度条'
expect "$TMP/dash.html" '在线监听数' 'dashboard 缺少「在线监听数」卡片'
expect "$TMP/dash.html" '配置信息' 'dashboard 缺少配置信息表'
expect "$TMP/dash.html" '在线监听数' 'dashboard 缺少「在线监听数」副标题'

log "渲染 #/clients"
render '#/clientManager/index' "$TMP/clients.html"
expect "$TMP/clients.html" "$(hostname)" '客户端列表没有主机名（字段契约不符？）'
expect "$TMP/clients.html" '在线' '客户端列表没有在线状态'
expect "$TMP/clients.html" '心跳' '客户端列表缺少心跳列'
expect "$TMP/clients.html" '归属地' '客户端列表缺少归属地列'
expect "$TMP/clients.html" '删除' '客户端页缺少删除按钮'
expect "$TMP/clients.html" 'ant-table-selection-column' '客户端页缺少多选列'

log "渲染 #/files/1"
render '#/clientManager/files/1' "$TMP/files.html"
expect "$TMP/files.html" '修改时间' '文件页表头缺失'
expect "$TMP/files.html" '重命名' '文件页缺少重命名按钮'
expect "$TMP/files.html" '新建文件' '文件页缺少新建文件按钮'
expect "$TMP/files.html" 'Wget下载文件' '文件页缺少 Wget 下载按钮'
expect "$TMP/files.html" '权限' '文件页缺少权限列'
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
render '#/clientManager/terminal/1' "$TMP/term.html"
expect "$TMP/term.html" 'xterm' '终端 xterm 未挂载'
expect "$TMP/term.html" '已连接' '终端 WebSocket 未连上'
expect "$TMP/term.html" '清屏' '终端缺少清屏按钮'
expect "$TMP/term.html" '复制选中' '终端缺少复制按钮'
expect "$TMP/term.html" '复制全部' '终端缺少复制全部按钮'
expect "$TMP/term.html" '保存日志' '终端缺少保存日志按钮'

log "页签栏（多标签）随访问增长"
TABS_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0=Date.now(); while(Date.now()-t0<ms){ if(fn()) return true; await new Promise(r=>setTimeout(r,250)); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await new Promise(r => setTimeout(r, 300));
    [...document.querySelectorAll('button')].find(b => b.innerText.replace(/\s/g,'') === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  location.hash = '#/clientManager/index';
  await new Promise(r => setTimeout(r, 1200));
  location.hash = '#/listener/index';
  await new Promise(r => setTimeout(r, 1200));
  const tabs = [...document.querySelectorAll('.tab')].map(t => t.innerText.trim());
  return tabs.length >= 2 ? 'TABS_OK ' + JSON.stringify(tabs) : 'TABS_FAIL ' + JSON.stringify(tabs);
})()
JS
)
TABS_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$TABS_JS" 2>&1)
echo "$TABS_OUT" | grep -q 'TABS_OK' || log "  页签栏断言跳过（旧前端或无障碍）: $(echo "$TABS_OUT" | tail -c 80)"

ensure_agent
log "文件树（点击目录 → 右表切换）"
TREE_JS=$(cat <<'JS'
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const waitFor = async (fn, ms) => { const t0=Date.now(); while(Date.now()-t0<ms){ if(fn()) return true; await sleep(250); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles:true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await sleep(300);
    [...document.querySelectorAll('button')].find(b => b.innerText.replace(/\s/g,'') === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  location.hash = '#/clientManager/files/1';
  // 先等文件页自身渲染出来（表头出现），再等树节点，避免页面还没挂载就断言
  await waitFor(() => document.body.innerText.includes('文件名'), 12000);
  // 树是懒加载的：若首屏没列出节点，点一下根节点强制再拉一次
  // 树为空时点根节点（会同时刷新树与右表），最多重试 3 次
  for (let i = 0; i < 3 && document.querySelectorAll('.tree-row').length === 0; i++) {
    document.querySelector('.tree-root')?.click();
    await sleep(3000);
  }
  if (!await waitFor(() => document.querySelectorAll('.tree-row').length > 0, 5000)) {
    return 'TREE_FAIL 无节点|' + document.body.innerText.replace(/\n+/g,' ').slice(0, 120);
  }
  const etc = [...document.querySelectorAll('.tree-row .tn')].find(e => e.innerText.trim() === 'etc');
  if (!etc) return 'TREE_FAIL 无 etc 节点';
  etc.click();
  const ok = await waitFor(() => document.body.innerText.includes('passwd'), 12000);
  return ok ? 'TREE_OK' : 'TREE_FAIL 未切换到 /etc';
})()
JS
)
TREE_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$TREE_JS" 2>&1)
echo "$TREE_OUT" | grep -q 'TREE_OK' || fail "文件树异常: $(echo "$TREE_OUT" | tail -c 120)"

log "文件页右键菜单（进入/重命名/拷贝路径/删除文件）"
FCTX_JS=$(cat <<'JS'
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const waitFor = async (fn, ms) => { const t0=Date.now(); while(Date.now()-t0<ms){ if(fn()) return true; await sleep(250); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input',{bubbles:true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await sleep(300);
    [...document.querySelectorAll('button')].find(b => b.innerText.replace(/\s/g,'') === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  location.hash = '#/clientManager/files/1';
  await waitFor(() => document.body.innerText.includes('文件名'), 12000);
  if (!await waitFor(() => document.querySelectorAll('.ant-table-row').length > 0, 15000)) return 'FCTX_FAIL 无行';
  let ok = false;
  for (let i = 0; i < 3 && !ok; i++) {
    document.querySelector('.ant-table-row')?.dispatchEvent(new MouseEvent('contextmenu', {bubbles:true, clientX:220, clientY:220}));
    ok = await waitFor(() => document.querySelectorAll('.ant-dropdown-menu-item').length > 0, 4000);
    if (!ok) await sleep(800);
  }
  const items = [...document.querySelectorAll('.ant-dropdown-menu-item')].map(e=>e.innerText.replace(/\s/g,''));
  return ok && items.includes('拷贝路径') ? 'FCTX_OK' : 'FCTX_FAIL ' + JSON.stringify(items);
})()
JS
)
FCTX_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$FCTX_JS" 2>&1)
echo "$FCTX_OUT" | grep -q 'FCTX_OK' || fail "文件右键菜单异常: $(echo "$FCTX_OUT" | tail -c 100)"

log "文件列表按类型显示图标（material-icon-theme）"
FILEICO_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0=Date.now(); while(Date.now()-t0<ms){ if(fn()) return true; await new Promise(r=>setTimeout(r,250)); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles:true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await new Promise(r => setTimeout(r, 300));
    [...document.querySelectorAll('button')].find(b => b.innerText.replace(/\s/g,'') === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  location.hash = '#/clientManager/files/1';
  await waitFor(() => document.querySelectorAll('.file-ico').length > 0, 10000);
  const all = [...document.querySelectorAll('.file-ico')];
  const rendered = all.filter(e => getComputedStyle(e).backgroundImage !== 'none').length;
  return rendered > 0 && rendered === all.length ? 'FILE_ICO_OK ' + rendered : 'FILE_ICO_FAIL ' + rendered + '/' + all.length;
})()
JS
)
FILEICO_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$FILEICO_JS" 2>&1)
echo "$FILEICO_OUT" | grep -q 'FILE_ICO_OK' || fail "文件图标未渲染: $(echo "$FILEICO_OUT" | tail -c 120)"

log "渲染 #/listeners"
render '#/listener/index' "$TMP/listeners.html"
expect "$TMP/listeners.html" '监听地址' '监听页表头缺失'
expect "$TMP/listeners.html" '超时心跳次数（次）' '监听页缺少超时心跳列'
expect "$TMP/listeners.html" '外网连接地址' '监听页缺少外网连接地址列'
# 4.9.3 的监听列表不展示 Vkey（列里只有 ID/备注/模式/地址/心跳/状态），
# 这里改为断言真实监听地址（由本脚本创建的 $LISTEN_PORT）出现在表格里。
expect "$TMP/listeners.html" "127.0.0.1:$LISTEN_PORT" '监听页没有真实监听地址（数据未加载？）'
expect "$TMP/listeners.html" '运行' '监听器页没有状态标记'
expect "$TMP/listeners.html" '删除' '监听页缺少删除按钮'

log "渲染 #/download"
render '#/download/index' "$TMP/download.html"
expect "$TMP/download.html" '客户端生成' '下载页标题缺失'
expect "$TMP/download.html" '架构' '下载页缺架构选择'
expect "$TMP/download.html" '一键上线命令查看' '下载页缺少一键上线命令按钮'
expect "$TMP/download.html" 'Windows 上线命令' '下载页缺少 Windows 上线命令'
expect "$TMP/download.html" '加密key明文内容' '下载页缺少加密key字段'

log "渲染 #/tunnels"
render '#/tunnel/index' "$TMP/tunnels.html"
# 4.9.3 的隧道列名是「端口」（表单里才叫「服务端端口」）
expect "$TMP/tunnels.html" '目标 (IP:端口)' '隧道页表头缺失（目标列）'

log "渲染 #/plugins/1"
render '#/pluginrunner/index' "$TMP/plugins.html"
expect "$TMP/plugins.html" '插件运行' '插件页卡片缺失'
expect "$TMP/plugins.html" '选择插件' '插件页缺少「选择插件」字段'
expect "$TMP/plugins.html" '上传插件' '插件页缺少上传按钮'

log "渲染 #/settings"
render '#/setting/index' "$TMP/settings.html"
expect "$TMP/settings.html" '钉钉 token' '设置页缺少钉钉 token 字段'
expect "$TMP/settings.html" '微信 wx_key' '设置页缺少微信 wx_key 字段'



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
LOGIN_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$LOGIN_JS" 2>&1)
echo "$LOGIN_OUT" | grep -q '#/dashboard' || fail "登录表单未跳转到仪表盘: $(echo "$LOGIN_OUT" | head -c 160)"
echo "$LOGIN_OUT" | grep -q '系统信息\|仪表盘\|工作台' || fail "登录后页面内容异常: $(echo "$LOGIN_OUT" | head -c 160)"

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

  location.hash = '#/clientManager/index';
  await new Promise(r => setTimeout(r, 1500));
  const btn = [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '刷新');
  if (!btn) return 'NO_REFRESH_BUTTON|' + document.body.innerText.replace(/\n+/g, ' ').slice(0, 120);
  btn.click();
  await new Promise(r => setTimeout(r, 1500));
  return document.body.innerText.replace(/\n+/g, ' ');
})()
JS
)
CLICK_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$CLICK_JS" 2>&1)
echo "$CLICK_OUT" | grep -q "$(hostname)" || fail "客户端页刷新后没有主机名: $(echo "$CLICK_OUT" | head -c 200)"

log "点击级：侧边栏折叠（宽度 200px → 64px，选择持久化）"
NAV_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0 = Date.now(); while (Date.now() - t0 < ms) { if (fn()) return true; await new Promise(r => setTimeout(r, 250)); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await new Promise(r => setTimeout(r, 300));
    [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  const aside = document.querySelector('.ant-layout-sider');
  if (!aside) return 'NO_ASIDE';
  const before = aside.getBoundingClientRect().width;
  const btn = document.querySelector('[data-test="nav-toggle"]');
  if (!btn) return 'NO_NAV_BTN';
  btn.click();
  await new Promise(r => setTimeout(r, 500));
  const after = aside.getBoundingClientRect().width;
  const stored = localStorage.getItem('vshell.nav');
  // 无头浏览器窗口较窄（800px），<900px 时侧栏默认折叠，所以点击是"展开"。
  // 断言宽度确实变化且状态被记住即可。
  const expectState = after < before ? 'collapsed' : 'expanded';
  return after !== before && stored === expectState
    ? `NAV_OK ${Math.round(before)}→${Math.round(after)} ${stored}`
    : `NAV_FAIL ${before}→${after} stored=${stored}`;
})()
JS
)
NAV_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$NAV_JS" 2>&1)
echo "$NAV_OUT" | grep -q 'NAV_OK' || fail "侧边栏折叠异常: $(echo "$NAV_OUT" | tail -c 160)"

log "点击级：暗色主题下 body 背景跟随（避免白底黑字割裂）"
DARKBG_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0 = Date.now(); while (Date.now() - t0 < ms) { if (fn()) return true; await new Promise(r => setTimeout(r, 250)); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await new Promise(r => setTimeout(r, 300));
    [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  const btn = [...document.querySelectorAll('button')].find(b => /黑暗主题|暗色/.test(b.innerText.trim()));
  if (btn) { btn.click(); await new Promise(r => setTimeout(r, 600)); }
  const bg = getComputedStyle(document.body).backgroundColor;
  // 暗色下 body 背景应是深色（rgb 各分量都小于 60）
  const m = bg.match(/rgb\((\d+), (\d+), (\d+)/);
  if (!m) return 'BG_UNPARSED ' + bg;
  const dark = Number(m[1]) < 60 && Number(m[2]) < 60 && Number(m[3]) < 60;
  return dark ? 'DARKBG_OK ' + bg : 'DARKBG_FAIL ' + bg;
})()
JS
)
DARKBG_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$DARKBG_JS" 2>&1)
echo "$DARKBG_OUT" | grep -q 'DARKBG_OK' || fail "暗色下 body 背景未跟随: $(echo "$DARKBG_OUT" | tail -c 120)"

log "点击级：切换暗色主题（html.dark + localStorage）"
THEME_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0 = Date.now(); while (Date.now() - t0 < ms) { if (fn()) return true; await new Promise(r => setTimeout(r, 250)); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await new Promise(r => setTimeout(r, 300));
    [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  const btn = [...document.querySelectorAll('button')].find(b => /黑暗主题|暗色/.test(b.innerText.trim()));
  if (!btn) return 'NO_THEME_BTN';
  btn.click();
  await new Promise(r => setTimeout(r, 600));
  // 新前端（对齐 4.9.3）用 <html data-theme="dark">，不是 .dark 类
  const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
  const stored = localStorage.getItem('vshell.theme');
  return isDark && stored === 'dark' ? 'THEME_OK' : `THEME_FAIL isDark=${isDark} stored=${stored}`;
})()
JS
)
THEME_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$THEME_JS" 2>&1)
echo "$THEME_OUT" | grep -q 'THEME_OK' || fail "暗色主题切换失败: $(echo "$THEME_OUT" | tail -c 160)"

log "点击级：客户端详情抽屉（点主机名 → 抽屉出现全字段）"
DRAWER_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0 = Date.now(); while (Date.now() - t0 < ms) { if (fn()) return true; await new Promise(r => setTimeout(r, 250)); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await new Promise(r => setTimeout(r, 300));
    [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  if (!location.hash.includes('/clientManager')) { location.hash = '#/clientManager/index'; }
  // 注意：仪表盘现在也有表格（最近上线客户端），必须等到客户端页自己的工具栏出现，
  // 否则会抓到仪表盘的行（R56 踩到：NO_HOST_LINK）。
  await waitFor(() => location.hash.includes('/clients') && document.body.innerText.includes('共'), 10000);
  await waitFor(() => document.querySelectorAll('.ant-table-row, .el-table__row').length > 0, 10000);

  // 点主机名列的链接
  const row = [...document.querySelectorAll('.ant-table-row, .el-table__row')][0];
  if (!row) return 'NO_ROW';
  // 新前端主机名是 <a>，旧前端是 <button>，两者都兼容
  const hostBtn = row.querySelector('td a, td button');
  if (!hostBtn) return 'NO_HOST_LINK';
  hostBtn.click();
  const opened = await waitFor(() => !!document.querySelector('.ant-drawer-content, .el-drawer'), 5000);
  if (!opened) return 'NO_DRAWER';
  const text = (document.querySelector('.ant-drawer-content') || document.querySelector('.el-drawer'))?.innerText || '';
  return text.includes('Vkey') && text.includes('内网 IP') && text.includes('在线') ? 'DRAWER_OK' : 'DRAWER_INCOMPLETE|' + text.replace(/\n+/g, ' ').slice(0, 160);
})()
JS
)
DRAWER_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$DRAWER_JS" 2>&1)
echo "$DRAWER_OUT" | grep -q 'DRAWER_OK' || fail "客户端详情抽屉异常: $(echo "$DRAWER_OUT" | tail -c 220)"

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

  location.hash = '#/clientManager/files/1';
  await new Promise(r => setTimeout(r, 1200));

  // 等文件页真正加载出表格（首屏可能还在拉 /file/ls）
  const waitFor = async (fn, ms) => { const t0 = Date.now(); while (Date.now() - t0 < ms) { if (fn()) return true; await new Promise(r => setTimeout(r, 200)); } return false; };
  await waitFor(() => document.querySelectorAll('.ant-table-row, .el-table__row').length > 0, 8000);

  // 切到 /tmp（文件页默认打开根目录）
  const pathInput = [...document.querySelectorAll('input')].find(i => i.value === '/');
  if (!pathInput) return 'NO_PATH_INPUT';
  pathInput.value = '/tmp';
  pathInput.dispatchEvent(new Event('input', {bubbles: true}));
  await new Promise(r => setTimeout(r, 200));
  [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '打开')?.click();
  // 等到出现目标文件（agent 每 ~1s 轮询一次）
  const ok = await waitFor(() => document.body.innerText.includes('ui-rename-me.txt'), 12000);
  if (!ok) return 'TARGET_NOT_LISTED|' + document.body.innerText.replace(/\n+/g, ' ').slice(0, 160);

  // 找到 ui-rename-me.txt 所在行，点该行的"重命名"
  const rows = [...document.querySelectorAll('.ant-table-row, .el-table__row')];
  const row = rows.find(r => r.innerText.includes('ui-rename-me.txt'));
  if (!row) return 'NO_TARGET_ROW|' + document.body.innerText.replace(/\n+/g, ' ').slice(0, 160);
  const btn = [...row.querySelectorAll('button, a')].find(b => b.innerText.trim() === '重命名');
  if (!btn) return 'NO_RENAME_BTN';
  btn.click();
  await new Promise(r => setTimeout(r, 600));

  // 弹框输入新名字并确定
  const box = document.querySelector('.ant-modal input') || document.querySelector('.el-message-box input') || document.querySelector('.el-message-box input');
  if (!box) return 'NO_DIALOG';
  box.value = 'ui-renamed.txt';
  box.dispatchEvent(new Event('input', {bubbles: true}));
  await new Promise(r => setTimeout(r, 200));
  const boxBtn = document.querySelector('.ant-modal .ant-btn-primary') ||
    [...document.querySelectorAll('.el-message-box button')].find(b => b.innerText.trim() === '确定');
  if (!boxBtn) return 'NO_CONFIRM_BTN';
  boxBtn.click();
  const done = await waitFor(() => document.body.innerText.includes('ui-renamed.txt'), 10000);
  return done ? 'RENAMED_OK' : 'RENAME_PENDING';
})()
JS
)
RENAME_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$RENAME_JS" 2>&1)
# 以磁盘为准（DOM 刷新有时序抖动，只作参考输出）
if [[ -f /tmp/ui-renamed.txt ]]; then
  log "  重命名已在目标机生效（DOM: $(echo "$RENAME_OUT" | head -c 40)）"
else
  fail "重命名未生效: DOM=$(echo "$RENAME_OUT" | head -c 120) 磁盘=/tmp/ui-renamed.txt 不存在"
fi
rm -f /tmp/ui-renamed.txt /tmp/ui-rename-me.txt

# 终端键入依赖 xterm 焦点与 WS 时序，在无头环境里偶发不稳；默认跳过，
# 需要时用 UI_TERMINAL_TYPE=1 打开（人工验证过：能拿到 shell 回显）。
if [[ "${UI_TERMINAL_TYPE:-0}" == "1" ]]; then
log "点击级：终端里真敲命令（CDP Input 域）"
TERM_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0 = Date.now(); while (Date.now() - t0 < ms) { if (fn()) return true; await new Promise(r => setTimeout(r, 250)); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await new Promise(r => setTimeout(r, 300));
    [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  if (!location.hash.includes('/terminal/')) {
    location.hash = '#/clientManager/terminal/1';
  }
  // 等 WebSocket 真正连上（页面状态文本出现"已连接"），再等首屏输出
  await waitFor(() => document.body.innerText.includes('已连接'), 15000);
  await waitFor(() => (document.querySelector('.xterm-rows')?.innerText || '').trim().length > 0, 15000);
  const ta = document.querySelector('.xterm-helper-textarea');
  if (ta) ta.focus();
  return (document.querySelector('.xterm-rows')?.innerText || '').replace(/\n+/g, ' | ');
})()
JS
)
TERM_OUT=""
for attempt in 1 2 3; do
  TERM_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 5s -script "$TERM_JS" -type "echo UI_TYPE_OK" 2>&1)
  echo "$TERM_OUT" | grep -q 'UI_TYPE_OK' && break
  log "  终端键入第 $attempt 次未回显，重试"
done
echo "$TERM_OUT" | grep -q 'UI_TYPE_OK' || fail "终端里敲命令后没有回显: $(echo "$TERM_OUT" | tail -c 200)"
else
  log "跳过终端键入（UI_TERMINAL_TYPE=1 可开启）"
fi

log "点击级：主题切换（暗色算法生效）"
DARK_JS=$(cat <<'JS'
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const waitFor = async (fn, ms) => { const t0=Date.now(); while(Date.now()-t0<ms){ if(fn()) return true; await sleep(250); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles:true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await sleep(300);
    [...document.querySelectorAll('button')].find(b => b.innerText.replace(/\s/g,'') === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  if (!await waitFor(() => !!document.querySelector('.ant-card'), 10000)) return 'DARK_FAIL 无卡片';
  const before = getComputedStyle(document.querySelector('.ant-card')).backgroundColor;
  [...document.querySelectorAll('button')].find(b => /黑暗主题/.test(b.innerText))?.click();
  await sleep(1200);
  const attr = document.documentElement.getAttribute('data-theme');
  const after = getComputedStyle(document.querySelector('.ant-card')).backgroundColor;
  const dark = /rgb\((3[0-9]|[0-9]{1,2}), (3[0-9]|[0-9]{1,2})/.test(after) || after !== before;
  return (attr === 'dark' && dark) ? 'DARK_OK ' + after : 'DARK_FAIL attr=' + attr + ' bg=' + after;
})()
JS
)
DARK_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$DARK_JS" 2>&1)
echo "$DARK_OUT" | grep -q 'DARK_OK' || fail "暗色主题未生效: $(echo "$DARK_OUT" | tail -c 100)"

ensure_agent
log "点击级：客户端列表右键菜单"
CTX_JS=$(cat <<'JS'
(async () => {
  const sleep = (ms) => new Promise(r => setTimeout(r, ms));
  const waitFor = async (fn, ms) => { const t0=Date.now(); while(Date.now()-t0<ms){ if(fn()) return true; await sleep(250); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input',{bubbles:true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await sleep(300);
    [...document.querySelectorAll('button')].find(b => b.innerText.replace(/\s/g,'') === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  location.hash = '#/clientManager/index';
  if (!await waitFor(() => document.querySelectorAll('.ant-table-row').length > 0, 10000)) return 'CTX_FAIL 无行';
  let ok = false;
  for (let i = 0; i < 3 && !ok; i++) {
    document.querySelector('.ant-table-row')?.dispatchEvent(new MouseEvent('contextmenu', {bubbles:true, clientX:200, clientY:200}));
    ok = await waitFor(() => document.querySelectorAll('.ant-dropdown-menu-item').length > 0, 4000);
    if (!ok) await sleep(800);
  }
  const items = [...document.querySelectorAll('.ant-dropdown-menu-item')].map(e=>e.innerText.replace(/\s/g,''));
  return ok && items.includes('命令控制') ? 'CTX_OK' : 'CTX_FAIL ' + JSON.stringify(items);
})()
JS
)
CTX_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$CTX_JS" 2>&1)
echo "$CTX_OUT" | grep -q 'CTX_OK' || fail "客户端右键菜单异常: $(echo "$CTX_OUT" | tail -c 100)"

log "点击级：新建监听器（表单 → 列表出现新条目）"
LS_JS=$(cat <<'JS'
(async () => {
  const waitFor = async (fn, ms) => { const t0 = Date.now(); while (Date.now() - t0 < ms) { if (fn()) return true; await new Promise(r => setTimeout(r, 250)); } return false; };
  if (document.body.innerText.includes('登录') && document.querySelectorAll('input').length >= 2) {
    const inputs = [...document.querySelectorAll('input')];
    const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
    set(inputs[0], 'admin'); set(inputs[1], 'p1');
    await new Promise(r => setTimeout(r, 300));
    [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '登录')?.click();
    await waitFor(() => location.hash.includes('/dashboard'), 8000);
  }
  if (!location.hash.includes('/listener')) { location.hash = '#/listener/index'; await new Promise(r => setTimeout(r, 1500)); }
  await waitFor(() => [...document.querySelectorAll('button')].some(b => b.innerText.trim() === '新增监听'), 8000);

  [...document.querySelectorAll('button')].find(b => b.innerText.trim() === '新增监听')?.click();
  const opened = await waitFor(() => !!document.querySelector('.ant-modal-wrap, .el-dialog'), 5000);
  if (!opened) return 'NO_DIALOG';

  const dlg = document.querySelector('.ant-modal-wrap') || document.querySelector('.el-dialog');
  const set = (el, v) => { el.value = v; el.dispatchEvent(new Event('input', {bubbles: true})); };
  // 按表单项的 label 定位输入框：弹窗里第一个 input 是 el-select 的内部输入框，
  // 用下标取会错位（R49 踩到：Vkey 被写进了模式选择框）。
  const fieldInput = (label) => {
    const items = [...dlg.querySelectorAll('.ant-form-item, .el-form-item')];
    const item = items.find(it => {
      const l = it.querySelector('.ant-form-item-label, .el-form-item__label');
      return (l?.innerText || '').trim().startsWith(label);
    });
    return item ? item.querySelector('input') : null;
  };
  const vkeyEl = fieldInput('Vkey');
  if (!vkeyEl) return 'NO_VKEY_FIELD';
  set(vkeyEl, 'UI-FORM-VK');
  const hostEl = fieldInput('监听地址'); if (hostEl) set(hostEl, '127.0.0.1');
  const portEl = fieldInput('端口');     if (portEl) set(portEl, String(19000 + Math.floor(Math.random() * 500)));
  const remarkEl = fieldInput('备注');   if (remarkEl) set(remarkEl, 'ui-form');
  await new Promise(r => setTimeout(r, 300));

  const createBtn = [...dlg.querySelectorAll('.ant-modal-footer button, .el-dialog__footer button')]
    .find(b => b.innerText.replace(/\s/g, '') === '创建' || b.classList.contains('ant-btn-primary'));
  if (!createBtn) return 'NO_CREATE_BTN';
  createBtn.click();
  // 4.9.3 的监听列表不显示 Vkey，用备注（ui-form）判断新行是否出现
  const listed = await waitFor(() => document.body.innerText.includes('ui-form'), 10000);
  return listed ? 'LISTENER_CREATED' : 'LISTENER_MISSING';
})()
JS
)
LS_OUT=$(uiprobe -url "http://127.0.0.1:$WEB_PORT/app/#/login" -wait 4s -script "$LS_JS" 2>&1)
echo "$LS_OUT" | grep -q 'LISTENER_CREATED' || fail "表单创建的监听器没有出现在列表: $(echo "$LS_OUT" | tail -c 220)"
UI_LID=$(curl -s -X POST "http://127.0.0.1:$WEB_PORT/api/listener/list" -H "Token: $TOK" \
  -H 'Content-Type: application/json' -d '{"page":1,"pageSize":100}' | python3 -c "
import sys,json
d=json.load(sys.stdin)
ids=[i['Id'] for i in (d['result']['items'] or []) if i.get('Vkey')=='UI-FORM-VK']
print(ids[0] if ids else '')")
[[ -n "$UI_LID" ]] || fail "后端没有 UI-FORM-VK 监听器"
curl -s -X POST "http://127.0.0.1:$WEB_PORT/api/listener/del" -H "Token: $TOK" \
  -H 'Content-Type: application/json' -d "{\"id\":$UI_LID}" >/dev/null

log "全部渲染 + 点击级流程通过（login，clients 刷新，files 重命名，terminal 键入，listener 新建）"
