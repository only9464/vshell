#!/usr/bin/env bash
# 抓取原版实例的黑盒响应，作为契约基线（fixtures）。
#
# 用法：
#   FS_BASE=http://127.0.0.1:39082 FS_USER=admin FS_PASS=123456 scripts/probe-original.sh
#
# 输出：testdata/original/<name>.json（已脱敏：token 换成 "<TOKEN>"）
#
# 说明：
#   - 只做只读探测：除 login 外不发任何写操作，不创建监听器/客户端；
#   - 带 id=1 的客户端相关接口只会得到 "connection error" 一类错误响应，
#     这正是我们要对齐的**错误形态**；
#   - 结果用于 controllers/contract_test.go 的键集合比对。
set -uo pipefail

BASE=${FS_BASE:-http://127.0.0.1:39082}
USER_NAME=${FS_USER:-admin}
PASS=${FS_PASS:-123456}
OUT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/testdata/original

mkdir -p "$OUT"

login_body=$(printf '{"username":"%s","password":"%s"}' "$USER_NAME" "$PASS")
login=$(curl -sS -m 10 -X POST "$BASE/api/login" -H 'Content-Type: application/json' -d "$login_body")
echo "$login" | python3 -c 'import sys,json; print(json.load(sys.stdin)["result"]["token"])' > /tmp/fs.token 2>/dev/null
TOKEN=$(cat /tmp/fs.token 2>/dev/null)

if [[ -z "${TOKEN:-}" ]]; then
  echo "login failed: $login" >&2
  exit 1
fi

save() { # save <name> <method> <path> [json]
  local name=$1 method=$2 path=$3 body=${4:-}
  local raw
  if [[ "$method" == GET ]]; then
    raw=$(curl -sS -m 10 -H "Token: $TOKEN" "$BASE$path")
  else
    raw=$(curl -sS -m 10 -X POST -H "Token: $TOKEN" -H 'Content-Type: application/json' -d "$body" "$BASE$path")
  fi
  printf '%s' "$raw" | python3 "$(dirname "${BASH_SOURCE[0]}")/lib/sanitize_json.py" > "$OUT/$name.json"
  printf '%-24s %s\n' "$name" "$(head -c 100 "$OUT/$name.json" | tr '\n' ' ')"
}

save login              POST /api/login          "$login_body"
save userinfo           GET  /api/getUserInfo
save dashboard          GET  /api/dashboard/info
save client_list        POST /api/client/list    '{"page":1,"pageSize":10}'
save listener_list      POST /api/listener/list  '{"page":1,"pageSize":10}'
save tunnel_list        POST /api/tunnel/list    '{"page":1,"pageSize":10}'
save runner_list        GET  /api/runner/list
save setting_get        GET  /api/setting/get
save screenshot_missing GET  '/api/screenshot/get?id=1&quality=normal'
save getdisk_missing    POST /api/file/getdisk   '{"id":1}'
save terminal_missing   POST /api/terminal/shell '{"id":1,"command":"id"}'
save install_missing    POST /api/install/install '{"id":1,"name":"x"}'
save download_stage     GET  '/api/download/stage?id=1&arch=windows_amd64'
save download_listen    GET  '/api/download/listen?arch=windows_amd64&tp=tcp&host=127.0.0.1&port=4444&vkey=vk&salt=s1&upx=false'
save menu_404           GET  /api/getMenuList
save permcode_404       GET  /api/getPermCode

rm -f /tmp/fs.token
echo "fixtures written to $OUT"
