#!/usr/bin/env python3
"""把 4.9.3 的静态产物（static/）本地伺服，并把 /api/* 代理到真实实例 :39082
（自动带 Basic 认证），从而可以在浏览器里真实渲染 4.9.3 的前端。"""
import base64
import http.server
import os
import socketserver
import sys
import urllib.request

STATIC = sys.argv[1] if len(sys.argv) > 1 else "static"
PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 18990
UPSTREAM = sys.argv[3] if len(sys.argv) > 3 else "http://127.0.0.1:39082"
AUTH = "Basic " + base64.b64encode(b"admin:123456").decode()

class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *a, **kw):
        super().__init__(*a, directory=STATIC, **kw)

    def log_message(self, fmt, *args):
        pass

    def _proxy(self, method):
        length = int(self.headers.get("Content-Length") or 0)
        body = self.rfile.read(length) if length else None
        req = urllib.request.Request(UPSTREAM + self.path, data=body, method=method)
        for h in ("Content-Type", "Token", "token"):
            if self.headers.get(h):
                req.add_header(h, self.headers.get(h))
        req.add_header("Authorization", AUTH)
        try:
            with urllib.request.urlopen(req, timeout=30) as r:
                data = r.read()
                self.send_response(r.status)
                for k, v in r.headers.items():
                    if k.lower() in ("content-length", "transfer-encoding", "connection"):
                        continue
                    self.send_header(k, v)
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)
        except urllib.error.HTTPError as e:
            data = e.read()
            self.send_response(e.code)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except Exception as e:
            msg = ("proxy error: %s" % e).encode()
            self.send_response(502)
            self.send_header("Content-Length", str(len(msg)))
            self.end_headers()
            self.wfile.write(msg)

    def do_GET(self):
        if self.path.startswith("/api/"):
            return self._proxy("GET")
        if self.path == "/" or self.path.startswith("/index.html"):
            self.path = "/index.html"
        return super().do_GET()

    def do_POST(self):
        if self.path.startswith("/api/"):
            return self._proxy("POST")
        self.send_error(404)

    def do_PUT(self):
        return self._proxy("PUT")

class Reuse(socketserver.ThreadingTCPServer):
    allow_reuse_address = True

if __name__ == "__main__":
    with Reuse(("127.0.0.1", PORT), Handler) as httpd:
        print("serving %s on %d -> %s" % (STATIC, PORT, UPSTREAM), flush=True)
        httpd.serve_forever()
