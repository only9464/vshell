#!/usr/bin/env python3
"""定位"某个字符串被哪个函数引用"——纯静态、无需 IDA 全量分析。

思路：
  1. 在 .rodata 里找目标字符串的地址；
  2. 扫 .text 里的 RIP 相对寻址指令（LEA/MOV r64, [rip+disp32]），算出目标地址；
  3. 目标命中字符串地址 → 用 .gopclntab 的函数表把指令地址归到所属函数。

这样即使二进制被 garble 混淆（名字随机、魔数被改），也能凭"字符串 → xref →
函数"把关键行为定位到具体地址，再用 objdump/Ghidra/IDA 只看这几个函数即可。

用法：
    scripts/string-xrefs.py /home/kali/go/bin/vshell "stageless/" "ebpf_" "conf"
    scripts/string-xrefs.py <bin> --list-conf   # 额外打印命中的字符串常量

输出：
    [pattern] "<命中的字符串>" @<字符串地址>
        <函数地址> <函数名>   (xref 指令地址)
"""

import argparse
import bisect
import struct
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from importlib import import_module

gosym = import_module("go-symbols".replace("-", "_")) if False else None


def find_section(blob, name):
    if blob[:4] != b"\x7fELF" or blob[4] != 2:
        raise SystemExit("not an ELF64 file")
    e_shoff = struct.unpack_from("<Q", blob, 0x28)[0]
    e_shentsize, e_shnum, e_shstrndx = struct.unpack_from("<HHH", blob, 0x3A)
    shstr_off = struct.unpack_from("<Q", blob, e_shoff + e_shstrndx * e_shentsize + 0x18)[0]
    for i in range(e_shnum):
        base = e_shoff + i * e_shentsize
        sh_name = struct.unpack_from("<I", blob, base)[0]
        sh_addr, sh_offset, sh_size = struct.unpack_from("<QQQ", blob, base + 0x10)
        end = blob.find(b"\x00", shstr_off + sh_name)
        sec_name = blob[shstr_off + sh_name:end].decode("utf-8", "replace")
        if sec_name == name:
            return sh_offset, sh_size, sh_addr
    raise SystemExit(f"section {name!r} not found")


def load_symbols(blob):
    off, size, _ = find_section(blob, ".gopclntab")
    tab = blob[off:off + size]
    nfunc, _nfiles = struct.unpack_from("<QQ", tab, 8)
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


def rodata_strings(blob, base_addr, data):
    """扫描 .rodata，返回 {addr: bytes}（可打印串，长度 ≥4）。"""
    out = {}
    i, n = 0, len(data)
    while i < n:
        b = data[i]
        if 32 <= b < 127 or b >= 0x80:
            j = i
            while j < n and (32 <= data[j] < 127 or data[j] >= 0x80):
                j += 1
            if j - i >= 4:
                out[base_addr + i] = data[i:j]
            i = j
        else:
            i += 1
    return out


def rip_relative_targets(blob, text_off, text_size, text_addr):
    """扫 .text，产出 (instr_addr, target_addr)。"""
    data = blob[text_off:text_off + text_size]
    prefixes = (b"\x48\x8d", b"\x4c\x8d", b"\x48\x8b", b"\x4c\x8b")
    for pref in prefixes:
        pos = 0
        while True:
            pos = data.find(pref, pos)
            if pos < 0:
                break
            if pos + 7 > len(data):
                break
            modrm = data[pos + 2]
            if (modrm & 0xC7) == 0x05:  # mod=00, rm=101 → RIP 相对
                disp = struct.unpack_from("<i", data, pos + 3)[0]
                instr_addr = text_addr + pos
                yield instr_addr, instr_addr + 7 + disp
            pos += 1


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("binary")
    ap.add_argument("patterns", nargs="*", help="要在字符串常量里查找的子串")
    ap.add_argument("--limit", type=int, default=6, help="每个 pattern 最多打印几条")
    args = ap.parse_args()

    blob = open(args.binary, "rb").read()
    syms = load_symbols(blob)
    sym_addrs = [a for a, _ in syms]

    def owner(addr):
        i = bisect.bisect_right(sym_addrs, addr) - 1
        if i >= 0:
            return syms[i]
        return (0, "")

    ro_off, ro_size, ro_addr = find_section(blob, ".rodata")
    strings = rodata_strings(blob, ro_addr, blob[ro_off:ro_off + ro_size])

    # 反向索引：字符串起始地址 → 文本
    wanted = {}
    for pat in args.patterns:
        p = pat.encode()
        for addr, s in strings.items():
            if p in s:
                # 只关心"字符串开头附近"的地址：xref 通常指向整个字面量的起点，
                # 因此把 pattern 在串内的偏移也纳入候选。
                off = s.find(p)
                wanted[addr + off] = (pat, s[:120])

    if not wanted:
        print("no matching strings found", file=sys.stderr)
        return

    tx_off, tx_size, tx_addr = find_section(blob, ".text")
    hits = {k: [] for k in wanted}
    wanted_addrs = set(wanted)
    for instr_addr, target in rip_relative_targets(blob, tx_off, tx_size, tx_addr):
        if target in wanted_addrs:
            hits[target].append(instr_addr)

    seen = set()
    for addr in sorted(wanted):
        pat, s = wanted[addr]
        func_addrs = hits.get(addr) or []
        if not func_addrs:
            continue
        print(f'[{pat}] "{s}" @{addr:#x}')
        shown = 0
        for ia in func_addrs:
            fa, fn = owner(ia)
            key = (fa, fn, pat)
            if key in seen:
                continue
            seen.add(key)
            print(f"    {fa:#x} {fn or '<unnamed>'}    (xref @{ia:#x})")
            shown += 1
            if shown >= args.limit:
                break


if __name__ == "__main__":
    main()
