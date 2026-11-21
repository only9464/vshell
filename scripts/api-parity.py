#!/usr/bin/env python3
"""对比"原版前端用到的 API 路径"与"复刻端注册的路由"，输出差集。

数据来源：
  - 原版：static/assets/*.js（提取版 SPA 的请求路径）
  - 复刻端：router/router.go 里的 /api/... 字符串

用法：python3 scripts/api-parity.py [--verbose]
"""
import re
import sys
import glob
import os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# 原版前端里这些不是 API（是页面路由/静态资源），比较时排除
NON_API = {
    "/dist/", "/assets/", "/src/",
}


def original_paths():
    paths = set()
    for f in glob.glob(os.path.join(ROOT, "static/assets/*.js")):
        try:
            s = open(f, encoding="utf-8", errors="ignore").read()
        except OSError:
            continue
        # url: "/xxx/yyy" 以及 axios.post("/xxx/yyy")
        for m in re.finditer(r'["\'`](/[a-z][a-zA-Z0-9_]+/[a-zA-Z0-9_/]*)["\'`]', s):
            paths.add(m.group(1))
    out = set()
    for p in paths:
        if p.startswith("/api/"):
            out.add(p[4:])
            continue
        if any(p.startswith(x) for x in NON_API):
            continue
        # 页面路由以 /index 结尾（如 /dashboard/index），不是 API
        if p.endswith("/index"):
            continue
        out.add(p)
    return out


def replica_paths():
    src = open(os.path.join(ROOT, "router/router.go"), encoding="utf-8").read()
    paths = set()
    for m in re.finditer(r'"/api(/[a-zA-Z0-9_/]+)"', src):
        paths.add(m.group(1))
    for m in re.finditer(r'beego\.Router\("/api(/[a-zA-Z0-9_/]+)"', src):
        paths.add(m.group(1))
    return paths


def main():
    verbose = "--verbose" in sys.argv
    orig = original_paths()
    ours = replica_paths()
    missing = sorted(p for p in orig - ours)
    extra = sorted(p for p in ours - orig)
    print(f"原版 API 路径: {len(orig)}    复刻端: {len(ours)}")
    print(f"缺失 {len(missing)} 个，多出 {len(extra)} 个")
    if missing:
        print("\n缺失（原版有、复刻端没有）:")
        for p in missing:
            print("  ", p)
    if extra and verbose:
        print("\n多出（复刻端有、原版前端未用）:")
        for p in extra:
            print("  ", p)
    return 1 if missing else 0


if __name__ == "__main__":
    sys.exit(main())
