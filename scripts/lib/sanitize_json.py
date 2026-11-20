#!/usr/bin/env python3
"""把抓到的原始响应脱敏后格式化成 JSON（读 stdin，写 stdout）。"""
import json
import re
import sys

raw = sys.stdin.read()
raw = re.sub(r"eyJ[A-Za-z0-9_\-\.]+", "<TOKEN>", raw)
try:
    obj = json.loads(raw)
except Exception:
    obj = {"_raw": raw[:400]}
print(json.dumps(obj, ensure_ascii=False, indent=2))
