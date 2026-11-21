#!/usr/bin/env python3
"""在 Go 二进制里定位"字符串 → 代码引用 → 所属函数"的对应关系。

用于 garble 过的二进制：函数名可能是乱的，但字符串字面量仍是明文（拼在一起），
Go 编译器用 `LEA reg, [rip+disp]` 取字面量地址，因此可以用"目标地址 == 字符串偏移"
反查引用点，再用 pclntab 找到所属函数。

用法：
  python3 scripts/find-string-ref.py BINARY 字符串 [--max-refs N]
"""
import argparse
import bisect
import re
import struct
import subprocess
import sys
import os

HERE = os.path.dirname(os.path.abspath(__file__))


def load_functions(binary):
    """用 go-symbols.py 取函数表（地址 + 名字）。"""
    out = subprocess.run([sys.executable, os.path.join(HERE, "go-symbols.py"), binary],
                         capture_output=True, text=True).stdout
    funcs = []
    for line in out.splitlines():
        parts = line.split(" ", 1)
        if len(parts) != 2 or not parts[0].startswith("00"):
            continue
        try:
            addr = int(parts[0], 16)
        except ValueError:
            continue
        funcs.append((addr, parts[1].strip()))
    funcs.sort()
    return funcs


def rip_refs(data, target):
    """扫描 `lea reg, [rip+disp32]`（REX 0x48/0x4c + 8d modrm 05/0d/15/1d/25/2d/35/3d）。"""
    hits = []
    modrm = {0x05, 0x0d, 0x15, 0x1d, 0x25, 0x2d, 0x35, 0x3d}
    for m in re.finditer(rb'[\x48\x4c]\x8d', data):
        i = m.start()
        if i + 7 > len(data):
            continue
        if data[i + 2] not in modrm:
            continue
        disp = struct.unpack_from("<i", data, i + 3)[0]
        # rip 相对：下一条指令 = i+7
        if i + 7 + disp == target:
            hits.append(i)
    return hits


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("binary")
    ap.add_argument("text")
    ap.add_argument("--max-refs", type=int, default=8)
    args = ap.parse_args()

    data = open(args.binary, "rb").read()
    needle = args.text.encode()
    offs = [m.start() for m in re.finditer(re.escape(needle), data)]
    if not offs:
        print(f"未找到字符串 {args.text!r}")
        return 1
    print(f"{args.text!r} 出现 {len(offs)} 次：{[hex(o) for o in offs[:8]]}")

    funcs = load_functions(args.binary)
    addrs = [a for a, _ in funcs]

    def owner(addr):
        i = bisect.bisect_right(addrs, addr) - 1
        return funcs[i] if i >= 0 else (0, "?")

    total = 0
    for off in offs:
        for ref in rip_refs(data, off):
            faddr, fname = owner(ref)
            print(f"  ref @{ref:#x} → {fname or '?'} ({faddr:#x})")
            total += 1
            if total >= args.max_refs:
                return 0
    if total == 0:
        print("  没有找到直接引用（字面量可能通过表/间接寻址访问）")
    return 0


if __name__ == "__main__":
    sys.exit(main())
