#!/usr/bin/env python3
"""从 Go ELF 二进制里恢复函数符号表（绕过 garble 的魔数改写）。

背景：本项目的原版参考二进制 `/home/kali/go/bin/vshell` 是 garble 混淆过的
Go 1.2x 构建；`go tool nm` 报 "no symbols"（ELF symtab 被剥离），IDA 对
500 MB 的 Go 二进制做全量分析代价极高。但 **`.gopclntab` 段仍在**，里面保存
着 38645 个函数的 `entryOff → nameOff` 映射；garble 只是把头部魔数改成了非标准
值（实测 0x840c4ea6），并给用户包随机化了名字。

本脚本不依赖 Go 版本字符串、也不校验魔数，只按已知的 pcHeader 字段宽度解析，
输出 `地址 名字` 两列，可直接和 objdump / IDA 的地址对齐。

用法：
    scripts/go-symbols.py /home/kali/go/bin/vshell > /tmp/syms.txt
    scripts/go-symbols.py <bin> --grep stageless

输出约定：
    <16 进制地址> <函数名>      —— 名字为空表示该函数的名字被 garble 抹掉
"""

import argparse
import struct
import sys


def find_section(blob, name):
    """解析 ELF64 节头，返回 (offset, size, addr)。"""
    if blob[:4] != b"\x7fELF" or blob[4] != 2:
        raise SystemExit("not an ELF64 file")
    e_shoff = struct.unpack_from("<Q", blob, 0x28)[0]
    e_shentsize, e_shnum, e_shstrndx = struct.unpack_from("<HHH", blob, 0x3A)
    shstr_off = struct.unpack_from("<Q", blob, e_shoff + e_shstrndx * e_shentsize + 0x18)[0]
    for i in range(e_shnum):
        base = e_shoff + i * e_shentsize
        sh_name, sh_type = struct.unpack_from("<II", blob, base)
        sh_addr, sh_offset, sh_size = struct.unpack_from("<QQQ", blob, base + 0x10)
        end = blob.find(b"\x00", shstr_off + sh_name)
        sec_name = blob[shstr_off + sh_name:end].decode("utf-8", "replace")
        if sec_name == name:
            return sh_offset, sh_size, sh_addr
    raise SystemExit(f"section {name!r} not found")


def parse_pclntab(tab):
    """返回 (nfunc, text_start, funcname_off, pcln_off)。"""
    # pcHeader：magic(4) pad(2) minLC(1) ptrSize(1) nfunc(8) nfiles(8)
    #           textStart(8) funcnameOffset(8) cuOffset(8) filetabOffset(8)
    #           pctabOffset(8) pclnOffset(8)
    ptr_size = tab[7]
    if ptr_size != 8:
        raise SystemExit(f"unsupported ptrSize {ptr_size}")
    nfunc, _nfiles = struct.unpack_from("<QQ", tab, 8)
    text_start, funcname_off = struct.unpack_from("<QQ", tab, 24)
    pcln_off = struct.unpack_from("<Q", tab, 64)[0]
    return nfunc, text_start, funcname_off, pcln_off


def read_cstr(buf, off):
    end = buf.find(b"\x00", off)
    if end < 0:
        return ""
    return buf[off:end].decode("utf-8", "replace")


def symbols(blob):
    off, size, _addr = find_section(blob, ".gopclntab")
    tab = blob[off:off + size]
    nfunc, text_start, funcname_off, pcln_off = parse_pclntab(tab)

    out = []
    for i in range(nfunc):
        try:
            entry_off, func_off = struct.unpack_from("<II", tab, pcln_off + i * 8)
        except struct.error:
            break
        fo = pcln_off + func_off
        if fo + 8 > len(tab):
            continue
        _e, name_off = struct.unpack_from("<Ii", tab, fo)
        name = ""
        if name_off >= 0 and funcname_off + name_off < len(tab):
            name = read_cstr(tab, funcname_off + name_off)
        out.append((text_start + entry_off, name))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("binary")
    ap.add_argument("--grep", help="只输出名字包含该子串的函数")
    ap.add_argument("--stats", action="store_true", help="按包前缀统计")
    args = ap.parse_args()

    blob = open(args.binary, "rb").read()
    syms = symbols(blob)

    if args.stats:
        from collections import Counter
        c = Counter(n.split(".")[0] for _, n in syms if n)
        for pref, cnt in c.most_common(30):
            print(f"{cnt:6d}  {pref}")
        print(f"total {len(syms)} functions, {sum(1 for _, n in syms if n)} named", file=sys.stderr)
        return

    for addr, name in syms:
        if args.grep and args.grep not in name:
            continue
        print(f"{addr:016x} {name}")


if __name__ == "__main__":
    main()
