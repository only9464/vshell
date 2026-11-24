#!/usr/bin/env python3
"""从 4.9.3 的打包产物（static/assets/*.js）里提取前端规格。

产出（默认写到 build/fe-spec/，不入库）：
  routes.json    路由表（path / name / title / icon / chunk）
  pages.json     每个页面 chunk 的列定义、表单 schema、菜单项、工具栏按钮、字符串、接口
  summary.md     人读的摘要（按页面列出提取结果）

用法：python3 scripts/extract-fe-spec.py [static 目录] [输出目录]
"""
import json
import os
import re
import sys

STATIC = sys.argv[1] if len(sys.argv) > 1 else "static"
OUT = sys.argv[2] if len(sys.argv) > 2 else "build/fe-spec"
ASSETS = os.path.join(STATIC, "assets")


def read(p):
    with open(p, encoding="utf-8", errors="ignore") as f:
        return f.read()


def js_files():
    for name in sorted(os.listdir(ASSETS)):
        if name.endswith(".js"):
            yield name, read(os.path.join(ASSETS, name))


def extract_routes(chunks):
    """路由：{path:"/x", name:"X", component:()=>import("./vX.js"), meta:{title,icon}}"""
    routes = []
    for name, s in chunks:
        for m in re.finditer(
            r'\{\s*path:"([^"]*)"\s*,\s*name:"([^"]+)"(.*?)\}', s, re.S):
            body = m.group(3)[:600]
            title = re.search(r'title:"([^"]{1,20})"', body)
            icon = re.search(r'icon:"([a-z0-9:.-]+)"', body)
            chunk = re.search(r'import\("\./(v[\w-]+\.js)"\)', body)
            routes.append({
                "path": m.group(1), "name": m.group(2),
                "title": title.group(1) if title else "",
                "icon": icon.group(1) if icon else "",
                "chunk": chunk.group(1) if chunk else "",
                "in": name,
            })
    return routes


def extract_columns(s):
    cols = []
    for m in re.finditer(r'\{\s*title:"([^"]{1,20})"\s*,\s*dataIndex:"([^"]+)"([^{}]{0,200})\}', s):
        extra = m.group(3)
        cols.append({
            "title": m.group(1), "dataIndex": m.group(2),
            "sorter": "sorter" in extra, "edit": "edit" in extra,
            "width": (re.search(r'width:(\d+)', extra) or [None, ""])[1] if re.search(r'width:(\d+)', extra) else "",
        })
    return cols


def extract_selects(s):
    """vxe 的 actionColumn：actions / dropDownActions 里的 {label, icon}"""
    out = []
    for m in re.finditer(r'\{(?:label:"([^"]{1,16})"|label:[\w.]+\(\s*"([^"]+)"\s*\))\s*,\s*icon:"([a-z0-9:.-]+)"', s):
        out.append({"label": m.group(1) or m.group(2), "icon": m.group(3)})
    return out


def extract_buttons(s):
    """工具栏按钮：preIcon:"x" 附近的按钮文案"""
    out = []
    for m in re.finditer(r'preIcon:"([a-z0-9:.-]+)"[^}]{0,120}?(?:C|I|_)\(" ?([^"]{1,16}?) ?"\)', s):
        out.append({"icon": m.group(1), "text": m.group(2).strip()})
    return out


def extract_schemas(s):
    """表单 schema：{component:"X", label:"Y", field:"z"} 或 {field:"z", label:"Y"}"""
    out = []
    for m in re.finditer(r'\{(?:component:"(\w+)",)?\s*label:"([^"]{1,20})",?\s*(?:field:"(\w+)")?', s):
        comp = m.group(1) or ""
        label = m.group(2)
        field = m.group(3) or ""
        if field or comp:
            out.append({"component": comp, "label": label, "field": field})
    return out


def extract_api(s):
    paths = set()
    for m in re.finditer(r'url:"(/[^"]+)"', s):
        paths.add(m.group(1))
    for m in re.finditer(r'"(/api/[a-z0-9/]+)"', s):
        paths.add(m.group(1))
    return sorted(paths)


def extract_cn_strings(s, limit=200):
    seen = []
    for m in re.finditer(r'"([\u4e00-\u9fa5][^"\\]{0,28})"', s):
        v = m.group(1)
        if v not in seen:
            seen.append(v)
        if len(seen) >= limit:
            break
    return seen


def main():
    chunks = list(js_files())
    routes = extract_routes(chunks)
    os.makedirs(OUT, exist_ok=True)
    with open(os.path.join(OUT, "routes.json"), "w", encoding="utf-8") as f:
        json.dump(routes, f, ensure_ascii=False, indent=2)

    by_chunk = {}
    for name, s in chunks:
        by_chunk[name] = {
            "size": len(s),
            "columns": extract_columns(s),
            "menus": extract_selects(s),
            "buttons": extract_buttons(s),
            "schemas": extract_schemas(s),
            "api": extract_api(s),
            "cn": extract_cn_strings(s),
        }
    with open(os.path.join(OUT, "pages.json"), "w", encoding="utf-8") as f:
        json.dump(by_chunk, f, ensure_ascii=False, indent=2)

    # 摘要：只输出"有页面特征"的 chunk（列 / 表单 / 菜单 / 按钮 非空）
    lines = ["# 4.9.3 前端规格摘要（自动提取）", ""]
    lines.append("## 路由表")
    lines.append("")
    lines.append("| path | name | title | icon | chunk |")
    lines.append("|---|---|---|---|---|")
    for r in sorted({(x["path"], x["name"], x["title"], x["icon"], x["chunk"]) for x in routes}):
        lines.append("| %s | %s | %s | %s | %s |" % r)
    lines.append("")
    lines.append("## 各 chunk 提取结果")
    for name, info in sorted(by_chunk.items(), key=lambda kv: -kv[1]["size"]):
        if not (info["columns"] or info["menus"] or info["buttons"] or info["schemas"]):
            continue
        lines.append("")
        lines.append("### `%s`（%d 字节）" % (name, info["size"]))
        if info["columns"]:
            lines.append("- 列：" + "、".join("%s(%s)" % (c["title"], c["dataIndex"]) for c in info["columns"]))
        if info["menus"]:
            lines.append("- 菜单/操作：" + "、".join("%s[%s]" % (m["label"], m["icon"]) for m in info["menus"][:12]))
        if info["buttons"]:
            lines.append("- 按钮：" + "、".join("%s[%s]" % (b["text"], b["icon"]) for b in info["buttons"][:12]))
        if info["schemas"]:
            lines.append("- 表单：" + "、".join(s["label"] for s in info["schemas"][:16]))
        if info["api"]:
            lines.append("- 接口：" + "、".join(info["api"][:10]))
    with open(os.path.join(OUT, "summary.md"), "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")
    print("routes:", len(routes), "chunks:", len(by_chunk), "->", OUT)


if __name__ == "__main__":
    main()
