#!/usr/bin/env python3
"""反汇编原版二进制的一段代码，并把调用/引用目标标上函数名。

用途：在 garble 混淆、IDA 全量分析代价过高的情况下，按"已知地址"做定点阅读。
地址来自 `scripts/go-symbols.py`（pclntab 恢复），例如：

    scripts/disas.py /home/kali/go/bin/vshell 0x1822d80 448      # ClientController.DelFile
    scripts/disas.py <bin> ClientController.DelFile              # 也可用符号名

输出：objdump 反汇编 + 每行末尾注释：
    <function+0xNN>      调用目标落在哪个函数
    lea "字符串"          RIP 相对寻址命中的字符串常量
"""

import argparse
import bisect
import os
import re
import struct
import subprocess
import sys


def find_section(blob, name):
    e_shoff = struct.unpack_from("<Q", blob, 0x28)[0]
    e_shentsize, e_shnum, e_shstrndx = struct.unpack_from("<HHH", blob, 0x3A)
    shstr_off = struct.unpack_from("<Q", blob, e_shoff + e_shstrndx * e_shentsize + 0x18)[0]
    for i in range(e_shnum):
        base = e_shoff + i * e_shentsize
        sh_name = struct.unpack_from("<I", blob, base)[0]
        sh_addr, sh_offset, sh_size = struct.unpack_from("<QQQ", blob, base + 0x10)
        end = blob.find(b"\x00", shstr_off + sh_name)
        if blob[shstr_off + sh_name:end].decode("utf-8", "replace") == name:
            return sh_offset, sh_size, sh_addr
    raise SystemExit(f"section {name!r} not found")


def load_symbols(blob):
    off, size, _ = find_section(blob, ".gopclntab")
    tab = blob[off:off + size]
    nfunc = struct.unpack_from("<Q", tab, 8)[0]
    text_start, funcname_off = struct.unpack_from("<QQ", tab, 24)
    pcln_off = struct.unpack_from("<Q", tab, 64)[0]

    def cstr(o):
        e = tab.find(b"\x00", o)
        return "" if e < 0 else tab[o:e].decode("utf-8", "replace")

    syms = []
    for i in range(nfunc):
        try:
            entry_off, func_off = struct.unpack_from("<II", tab, pcln_off + i * 8)
        except struct.error:
            break
        fo = pcln_off + func_off
        if fo + 8 > len(tab):
            continue
        _e, name_off = struct.unpack_from("<Ii", tab, fo)
        syms.append((text_start + entry_off, cstr(funcname_off + name_off) if name_off >= 0 else ""))
    syms.sort()
    return syms


def read_cstring(blob, addr, sections):
    for off, size, base in sections:
        if base <= addr < base + size:
            p = off + (addr - base)
            end = blob.find(b"\x00", p, p + 200)
            if end < 0:
                return None
            raw = blob[p:end]
            if len(raw) < 2:
                return None
            try:
                s = raw.decode("utf-8")
            except UnicodeDecodeError:
                return None
            if not all(32 <= ord(c) < 127 or ord(c) > 127 for c in s):
                return None
            return s
    return None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("binary")
    ap.add_argument("start", help="起始地址（0x...）或函数名")
    ap.add_argument("length", nargs="?", type=int, default=0, help="字节数（省略则取到下一个函数）")
    args = ap.parse_args()

    blob = open(args.binary, "rb").read()
    syms = load_symbols(blob)
    addrs = [a for a, _ in syms]

    if args.start.startswith("0x"):
        start = int(args.start, 16)
    else:
        cands = [(a, n) for a, n in syms if n.endswith(args.start)]
        if not cands:
            raise SystemExit(f"symbol {args.start!r} not found")
        start = cands[0][0]

    if args.length:
        length = args.length
    else:
        i = bisect.bisect_right(addrs, start)
        length = (addrs[i] - start) if i < len(addrs) else 4096

    out = subprocess.run(
        ["objdump", "-d", "--no-show-raw-insn", "-M", "intel",
         f"--start-address={start}", f"--stop-address={start + length}", args.binary],
        capture_output=True, text=True).stdout

    sections = []
    for name in (".rodata", ".data", ".noptrdata", ".text"):
        try:
            sections.append(find_section(blob, name))
        except SystemExit:
            pass

    def name_at(addr):
        i = bisect.bisect_right(addrs, addr) - 1
        if i >= 0 and addrs[i] <= addr:
            return syms[i][1] or "<unnamed>", addr - addrs[i]
        return None, 0

    line_re = re.compile(r"^\s*([0-9a-f]+):\s+(.*)$")
    rip_re = re.compile(r"#\s*([0-9a-f]+)\s")
    for line in out.splitlines():
        m = line_re.match(line)
        if not m:
            if line.strip().endswith(">:"):
                print(line)
            continue
        addr = int(m.group(1), 16)
        text = m.group(2).strip()
        note = ""
        tgt = rip_re.search(text)
        if tgt:
            t = int(tgt.group(1), 16)
            fn, off = name_at(t)
            if text.startswith("lea") or "lea" in text.split(",")[0]:
                s = read_cstring(blob, t, sections)
                if s:
                    note = f'  ; "{s}"'
            if not note and fn:
                note = f"  ; {fn}+0x{off:x}" if off else f"  ; {fn}"
        elif text.startswith(("call", "jmp", "j")) and re.search(r"\b[0-9a-f]{5,}\b", text):
            t = int(re.findall(r"\b([0-9a-f]{5,})\b", text)[-1], 16)
            fn, off = name_at(t)
            if fn:
                note = f"  ; {fn}+0x{off:x}" if off else f"  ; {fn}"
        print(f"{addr:08x}  {text}{note}")


if __name__ == "__main__":
    main()
