#!/usr/bin/env python3
"""从原版服务端二进制里提取**内嵌的 agent 模板**（PE / ELF）。

背景：本机原版 `/home/kali/go/bin/vshell` 有 502 MB，远超一个 Go 服务的体量——
差额就是内嵌的 agent 模板。实测扫到 40 个 PE 镜像（amd64/i386，console/GUI）
和多个 ELF 候选；它们就是面板"下载载荷"时要 patched 后发出去的东西
（对应文档里的 `stageless/<tp>_<arch>`、`dll/<tp>_<arch>` 路径规则）。

顺带：这些模板是**完整的 Go 二进制**，其 `.gopclntab` 也能用
`scripts/go-symbols.py` 恢复符号（实测 amd64 console 模板有 9482 个函数）。

用法：
    scripts/extract-templates.py /home/kali/go/bin/vshell --list
    scripts/extract-templates.py /home/kali/go/bin/vshell --out /tmp/templates
    scripts/extract-templates.py <bin> --out DIR --min-size 5000000
"""

import argparse
import os
import struct

ARCH = {0x14C: "i386", 0x8664: "amd64", 0xAA64: "arm64", 0x1C4: "arm"}
SUBSYS = {1: "native", 2: "gui", 3: "console"}


def pe_images(blob, min_size):
    """产出 (offset, arch, subsystem, size)。"""
    pos = 0
    while True:
        i = blob.find(b"MZ\x90\x00", pos)
        if i < 0:
            return
        pos = i + 1
        if i + 0x40 > len(blob):
            continue
        e_lfanew = struct.unpack_from("<I", blob, i + 0x3C)[0]
        pe = i + e_lfanew
        if pe + 24 > len(blob) or blob[pe:pe + 4] != b"PE\x00\x00":
            continue
        machine = struct.unpack_from("<H", blob, pe + 4)[0]
        nsec = struct.unpack_from("<H", blob, pe + 6)[0]
        opt_size = struct.unpack_from("<H", blob, pe + 20)[0]
        opt = pe + 24
        if opt + 72 > len(blob):
            continue
        subsystem = struct.unpack_from("<H", blob, opt + 68)[0]
        size_headers = struct.unpack_from("<I", blob, opt + 60)[0]
        sec = opt + opt_size
        end = size_headers
        for k in range(nsec):
            b = sec + k * 40
            if b + 40 > len(blob):
                break
            raw_size = struct.unpack_from("<I", blob, b + 16)[0]
            raw_off = struct.unpack_from("<I", blob, b + 20)[0]
            end = max(end, raw_off + raw_size)
        if end < min_size or i + end > len(blob):
            continue
        yield i, ARCH.get(machine, hex(machine)), SUBSYS.get(subsystem, str(subsystem)), end


def elf_images(blob, min_size):
    """产出 (offset, arch, '<elf>', size)；size 由 program headers 推算。"""
    pos = 0
    while True:
        i = blob.find(b"\x7fELF\x02\x01\x01", pos)
        if i < 0:
            return
        pos = i + 1
        if i + 64 > len(blob):
            continue
        e_machine = struct.unpack_from("<H", blob, i + 18)[0]
        e_phoff = struct.unpack_from("<Q", blob, i + 0x20)[0]
        e_phentsize = struct.unpack_from("<H", blob, i + 0x36)[0]
        e_phnum = struct.unpack_from("<H", blob, i + 0x38)[0]
        if not (0 < e_phnum < 64) or e_phoff + e_phnum * e_phentsize > len(blob):
            continue
        end = 0
        for k in range(e_phnum):
            b = i + e_phoff + k * e_phentsize
            p_offset = struct.unpack_from("<Q", blob, b + 0x08)[0]
            p_filesz = struct.unpack_from("<Q", blob, b + 0x20)[0]
            end = max(end, p_offset + p_filesz)
        if end < min_size or i + end > len(blob):
            continue
        arch = {62: "amd64", 3: "i386", 183: "arm64"}.get(e_machine, hex(e_machine))
        yield i, arch, "<elf>", end


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("binary")
    ap.add_argument("--out", help="提取目录（不指定则只列清单）")
    ap.add_argument("--min-size", type=int, default=500_000, help="最小镜像字节数")
    args = ap.parse_args()

    blob = open(args.binary, "rb").read()
    found = [
        r for r in list(pe_images(blob, args.min_size)) + list(elf_images(blob, args.min_size))
        if r[0] != 0  # offset 0 是外层二进制自身，不是内嵌模板
    ]
    found.sort(key=lambda r: -r[3])

    print(f"{'#':>2} {'offset':>12} {'kind':6} {'arch':6} {'subsys':8} {'size':>10}  name")
    for idx, (off, arch, sub, size) in enumerate(found):
        name = ""
        if args.out:
            os.makedirs(args.out, exist_ok=True)
            ext = ".exe" if sub != "<elf>" else ".elf"
            name = os.path.join(args.out, f"tpl_{idx:02d}_{arch}_{sub}{ext}")
            with open(name, "wb") as f:
                f.write(blob[off:off + size])
        print(f"{idx:2d} {off:#12x} {'PE' if sub != '<elf>' else 'ELF':6} {arch:6} {sub:8} {size:>10}  {name}")


if __name__ == "__main__":
    main()
