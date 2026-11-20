# VShell — 复刻研究（Reverse-Engineering Reimplementation）

> **Research notice**: an independent, unofficial reimplementation study of the VShell v3.0 C2
> framework, produced from binary analysis. Not affiliated with the original author, contains no
> original license, and does **not** claim protocol compatibility today. For authorized security
> research only.

**Documentation is the source of truth** — start here: [`docs/`](docs/README.md)

| Doc | What it answers |
|---|---|
| [docs/ROADMAP.md](docs/ROADMAP.md) | What is planned, in which order, with acceptance criteria |
| [docs/STATUS.md](docs/STATUS.md) | What actually works today (with `file:line` evidence) |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | Module layout, request path, code map |
| [docs/PROTOCOL.md](docs/PROTOCOL.md) | Wire formats: frame, KCP link, DNS, 24-byte records |
| [docs/API.md](docs/API.md) | Panel HTTP/WS contract + frontend field contract |
| [docs/FRONTEND.md](docs/FRONTEND.md) | Vue3 rewrite plan |
| [docs/DEV.md](docs/DEV.md) | Build / run / debug / test |
| [docs/RE-NOTES.md](docs/RE-NOTES.md) | Evidence register (hard evidence / inference / unrecovered) |

---

## Status at a glance (R1)

| Dimension | Level | Note |
|---|---|---|
| Code structure parity | ~75% | Packages, SQL schema, frames mirror the original |
| Panel usability | ~50% | Login/listeners/task queue real; dashboard/upload/plugin are shells |
| End-to-end delivery | ~15% | 7 gaps documented in [STATUS §4](docs/STATUS.md#4-端到端断点清单r1-实测) |
| Protocol interop with the official build | ~10% | Crypto frame aligned; command layer is our own text/JSON |

`go build ./...` and `go test ./...` pass (204 test functions). **Green tests do not mean the
product works end to end** — there is no cross-module integration test yet.

## Quick start

```bash
go build -o vshell-server .
mkdir -p conf && cat > conf/setting.conf <<'EOF'
master_type=web
web_port=8082
web_username=admin
web_password=admin123
web_jwt_secret=dev-secret
EOF
./vshell-server          # http://127.0.0.1:8082/
```

Agent (separate module, configuration injected at build time):

```bash
cd agent
go build -ldflags "\
  -X main.ServerAddr=http://127.0.0.1:8443 \
  -X main.VerifyKey=vk_demo \
  -X main.EncryptSalt=salt_demo" -o agent .
```

Known limitations of this quick start are listed in [docs/DEV.md §8](docs/DEV.md#8-常见坑).

## Layout

```
.
├── main.go          # server entry
├── router/          # HTTP/WS routing
├── controllers/     # panel API (beego controllers)
├── c2engine/        # C2 engine: listeners, protocol, payloads, storage, streams
├── utils/           # config, auth, license (removable), notify
├── agent/           # agent (standalone Go module)
├── static/          # original SPA assets (reference only; to be replaced by web/)
├── views/           # legacy templates (dead code, to be removed)
└── docs/            # ← source of truth
```

## Roadmap in one line

Make the chain work first (M0), align the protocol second (M1), make payloads real third (M2),
then capabilities (M3/M4), then the panel (M5) and the Vue3 rewrite (M6). Details and checkboxes:
[docs/ROADMAP.md](docs/ROADMAP.md).

## License

This repository does not include the original license; all rights of the original software belong
to its author. Research only — no commercial or unlawful use.
