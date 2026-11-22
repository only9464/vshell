# VShell — 复刻研究（逆向重实现）

> **研究声明**：本项目是基于二进制分析对 VShell v3.0 C2 框架的独立、非官方重实现研究。
> 与原作者无关联，不含原版许可，**不声称当前与官方版本协议互通**。仅限授权安全研究使用。

**文档即事实来源** —— 入口：[`docs/`](docs/README.md)

| 文档 | 回答什么 |
|---|---|
| [docs/ROADMAP.md](docs/ROADMAP.md) | 计划做什么、按什么顺序、验收标准是什么 |
| [docs/STATUS.md](docs/STATUS.md) | 现在到底能跑什么（附 `文件:行` 证据） |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 模块划分、请求路径、代码地图 |
| [docs/PROTOCOL.md](docs/PROTOCOL.md) | 线协议：传输帧、KCP 链路、DNS、24 字节记录 |
| [docs/API.md](docs/API.md) | 面板 HTTP/WS 契约 + 前端字段契约 |
| [docs/FRONTEND.md](docs/FRONTEND.md) | Vue3 重写方案 |
| [docs/CHECKLIST.md](docs/CHECKLIST.md) | 发版门禁与人工确认项 |
| [docs/RELEASE.md](docs/RELEASE.md) | 部署、上线客户端、备份升级 |
| [docs/DEV.md](docs/DEV.md) | 构建 / 运行 / 调试 / 测试 |
| [docs/MCP.md](docs/MCP.md) | 原生 MCP：`vshell-server mcp` + 13 个工具 |
| [docs/RE-NOTES.md](docs/RE-NOTES.md) | 逆向证据登记（硬证据 / 推断 / 未恢复） |

---

## 一眼看现状（R42）

| 维度 | 程度 | 说明 |
|---|---|---|
| 代码结构还原 | ✅ | 包结构、SQL schema、帧格式、记录词汇与原版逐项对照，证据见 [RE-NOTES](docs/RE-NOTES.md) |
| 面板可用性 | ✅ | Vue3 前端 13 个页面；**61 个 API 端点**，e2e 覆盖 55 个（其余为长连接与下载直链） |
| 端到端投递 | ✅ | **原生 TCP** / HTTP / KCP / WebSocket / DNS 五条传输 + 隧道数据面 + 插件执行 |
| 与官方版本协议互通 | ⚠️ | 帧层/认证/任务记录**逐字节对齐**（黄金向量测试）；官方 agent 能通过认证并上报 client 记录，`main` 通道仍未打通（[GAPS A-1](docs/GAPS.md)） |

**测试规模**：服务端 264 个测试函数 + agent 78 个 + e2e 129 条断言；
CI 跑 `go build`/`go vet`/`go test`/`go test -race` + 完整 e2e。

**已知不做的**（操作者明确）：通知（企业微信/钉钉）、OSS 上传、屏幕监控、
屏幕截图、**许可证**（均以空占位保留）；Windows/macOS 专有功能（当前环境为 Linux）。

## 快速开始

```bash
go build -o vshell-server .
mkdir -p conf && cat > conf/setting.conf <<'EOF'
master_type=web
web_port=8082
web_username=admin
web_password=admin123
web_jwt_secret=dev-secret
EOF
./vshell-server          # 浏览器打开 http://127.0.0.1:8082/
```

Agent（独立 module，配置在编译期注入）：

```bash
cd agent
go build -ldflags "\
  -X main.ServerAddr=http://127.0.0.1:8443 \
  -X main.VerifyKey=vk_demo \
  -X main.EncryptSalt=salt_demo" -o agent .
```

快速开始的已知限制（监听器端口、上线失败等）见 [docs/DEV.md §8](docs/DEV.md)。

## 目录结构

```
.
├── main.go          # 服务端入口
├── router/          # HTTP/WS 路由
├── controllers/     # 面板 API（beego 控制器）
├── c2engine/        # C2 引擎：监听器、协议、载荷、存储、流中继
├── utils/           # 配置、鉴权、许可证（可删）、通知
├── agent/           # Agent（独立 Go module）
├── static/          # 原版 SPA 资产（仅作参考，将由 web/ 取代）
├── views/           # 遗留模板（死代码，待删）
└── docs/            # ← 事实来源
```

## 路线图一句话

先把链路打通（M0），再对齐协议（M1），再把载荷做真（M2），然后补能力（M3/M4）、
补面板（M5）、重写前端（M6）。勾选清单与验收标准见 [docs/ROADMAP.md](docs/ROADMAP.md)。

## 许可

本仓库不含原版许可；原版软件的一切权利归其作者所有。**仅供研究，禁止商用或非法用途。**
