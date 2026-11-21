#!/usr/bin/env bash
# 官方 agent 互通探针：同一份官方载荷分别连"原版服务端"和"复刻端 official-tcp"，
# 统计 20 秒内各通道的注册次数，用于判断差异是协议问题还是时序 flaky。
#
# 用法：sudo ./scripts/official-probe.sh [轮数，默认 1]
set -uo pipefail
ROUNDS=${1:-1}
# 观察窗口（秒）：官方 agent 每 ~5s 重连一次，窗口越长统计越可信
WINDOW=${WINDOW:-20}
ROOT=$(cd "$(dirname "$0")/.." && pwd)
# 必须与原版二进制同一文件系统才能硬链（/tmp 常是 tmpfs），放在 $HOME 下。
WORK=$(mktemp -d "$HOME/.official-probe.XXXXXX")
ORIG_BIN=/home/kali/go/bin/vshell
ORIG_PORT=${ORIG_PORT:-39110}
OFF_PORT=${OFF_PORT:-18710}
LISTEN_PORT=${LISTEN_PORT:-18711}
VKEY=${VKEY:-PROBE-VK}

cleanup() {
  [[ -n "${ORIG_PID:-}" ]] && kill "$ORIG_PID" 2>/dev/null
  [[ -n "${OFF_PID:-}"  ]] && kill "$OFF_PID"  2>/dev/null
  [[ -n "${AGENT_PID:-}" ]] && kill "$AGENT_PID" 2>/dev/null
  if [[ "${KEEP:-0}" == "1" ]]; then
    echo "[probe] 保留现场：$WORK"
  else
    rm -rf "$WORK"
  fi
}
trap cleanup EXIT

log() { echo "[probe] $*"; }

if [[ ! -x "$ORIG_BIN" ]]; then
  log "缺少原版二进制 $ORIG_BIN，退出"
  exit 1
fi

# 1. 原版实例（硬链，读自己的 conf），只为生成载荷
mkdir -p "$WORK/orig/conf" "$WORK/orig/db"
ln -f "$ORIG_BIN" "$WORK/orig/vshell"
cat > "$WORK/orig/conf/setting.conf" <<CONF
master_type=web
web_port=$ORIG_PORT
web_ip=127.0.0.1
web_basic_auth=false
web_jwt_secret=probe
web_username=admin
web_password=123456
log_level=7
CONF
( cd "$WORK/orig" && exec ./vshell > s.log 2>&1 ) &
ORIG_PID=$!
sleep 3
TOK=$(curl -s -X POST "http://127.0.0.1:$ORIG_PORT/api/login" -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"123456"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["result"]["token"])')
if [[ -z "$TOK" ]]; then log "原版登录失败"; exit 1; fi

make_payload() { # $1=ConnectAddr  $2=输出路径
  curl -s -X POST "http://127.0.0.1:$ORIG_PORT/api/listener/add" -H "Token: $TOK" \
    -H 'Content-Type: application/json' \
    -d "{\"Mode\":\"tcp\",\"Vkey\":\"$VKEY\",\"Remark\":\"probe\",\"ListenAddr\":\"127.0.0.1:$LISTEN_PORT\",\"ConnectAddr\":\"$1\"}" >/dev/null
  local id
  id=$(curl -s -X POST "http://127.0.0.1:$ORIG_PORT/api/listener/list" -H "Token: $TOK" \
    -H 'Content-Type: application/json' -d '{"page":1,"pageSize":20}' \
    | python3 -c "import sys,json;d=json.load(sys.stdin);print([i['Id'] for i in d['result']['items'] if i['ConnectAddr']=='$1'][0])")
  curl -s -o "$2" "http://127.0.0.1:$ORIG_PORT/api/download/stageless?id=$id&arch=linux_amd64&token=$TOK"
  chmod +x "$2"
}

count() { # $1=日志文件 $2=关键字
  grep -c "$2" "$1" 2>/dev/null || true
}

# 2. 复刻端 official-tcp 服务端
# 每次都用当前源码重建，避免用到仓库里过期的 vshell-server（R59 踩到：
# 之前所有 A/B 结果其实来自 10 月 21 日的旧二进制，结论全部作废重跑）。
( cd "$ROOT" && go build -o "$WORK/vshell-server" . ) || { log "服务端构建失败"; exit 1; }
"$WORK/vshell-server" official-tcp -listen "127.0.0.1:$OFF_PORT" -vkey "$VKEY" -cmd id \
  > "$WORK/off.log" 2>&1 &
OFF_PID=$!
sleep 1

make_payload "127.0.0.1:$OFF_PORT" "$WORK/agent-official.elf" || { log "载荷生成失败"; exit 1; }
make_payload "127.0.0.1:$LISTEN_PORT" "$WORK/agent-orig.elf" || true

for r in $(seq 1 "$ROUNDS"); do
  : > "$WORK/off.log.run$r"
  log "第 $r 轮：官方 agent → 复刻端（${WINDOW}s）"
  ( cd "$WORK" && exec timeout "$WINDOW" ./agent-official.elf >/dev/null 2>&1 ) &
  AGENT_PID=$!
  sleep "$WINDOW"
  kill "$AGENT_PID" 2>/dev/null
  wait "$AGENT_PID" 2>/dev/null
  cp "$WORK/off.log" "$WORK/off.log.run$r"
  CONNS=$(count "$WORK/off.log" 'conn from')
  AUTHS=$(count "$WORK/off.log" 'authenticated')
  CONF=$(count "$WORK/off.log" 'client registered')
  SESS=$(count "$WORK/off.log" 'session issued')
  MAIN=$(count "$WORK/off.log" 'main channel registered')
  CHAN=$(count "$WORK/off.log" 'data channel registered')
  printf '  连接=%s 认证=%s conf=%s 会话=%s main=%s chan=%s\n' \
    "$CONNS" "$AUTHS" "$CONF" "$SESS" "$MAIN" "$CHAN"
  # conf 完成率 = 会话签发数 / 连接数（会话只在 conf 流程末尾签发）
  if [[ "$CONNS" -gt 0 ]]; then
    printf '  conf 完成率=%.0f%%（会话/连接）\n' "$(awk "BEGIN{printf 100*$SESS/$CONNS}")"
  fi
done

log "复刻端日志：$WORK/off.log.run*"
log "提示：原版服务端使用 $LISTEN_PORT，可用 $WORK/agent-orig.elf 做对照"
