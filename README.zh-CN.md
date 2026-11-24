# VShell — 复刻研究（逆向重实现）

> **研究声明**：本项目是基于二进制分析对 **VShell 4.9.3** 的独立、非官方重实现研究
> （基准 = 本机 `http://127.0.0.1:39082` 上运行的那一份）。
> 与原作者无关联，不含原版许可。仅限授权安全研究使用。

**文档即事实来源** —— 入口：[`docs/`](docs/README.md)

| 文档 | 回答什么 |
|---|---|
| [docs/ROADMAP.md](docs/ROADMAP.md) | 计划做什么、按什么顺序、验收标准是什么 |
| [docs/STATUS.md](docs/STATUS.md) | 现在到底能跑什么（附 `文件:行` 证据） |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 模块划分、请求路径、代码地图 |
| [docs/PROTOCOL.md](docs/PROTOCOL.md) | 线协议：传输帧、KCP 链路、DNS、24 字节记录 |
| [docs/API.md](docs/API.md) | 面板 HTTP/WS 契约 + 前端字段契约 |
| [docs/FRONTEND-493.md](docs/FRONTEND-493.md) | **按 4.9.3 复刻前端**：技术栈、路由/文案证据、逐页清单 |
| [docs/FRONTEND.md](docs/FRONTEND.md) | 上一版（Element Plus）前端说明 |
| [docs/CHECKLIST.md](docs/CHECKLIST.md) | 发版门禁与人工确认项 |
| [docs/RELEASE.md](docs/RELEASE.md) | 部署、上线客户端、备份升级 |
| [docs/DEV.md](docs/DEV.md) | 构建 / 运行 / 调试 / 测试 |
| [docs/MCP.md](docs/MCP.md) | 原生 MCP：`vshell-server mcp` + 13 个工具 |
| [docs/RE-NOTES.md](docs/RE-NOTES.md) | 逆向证据登记（硬证据 / 推断 / 未恢复） |

---

## 一眼看现状（R88）

| 维度 | 程度 | 说明 |
|---|---|---|
| 前端 | ✅ | **Ant Design Vue 新前端**（`web2/`，与 4.9.3 同栈：Vue3 + UnoCSS + Tailwind + antd），12 个页面；菜单/表头/按钮文案取自 4.9.3 打包产物；深色侧栏 + 多标签页签 + 文件树 + 按类型图标 |
| 后端 | ✅ | **原生 TCP 监听**（首选；e2e 覆盖 checkin/poll/命令/文件列表/隧道/插件）+ HTTP/HTTPS/KCP/WS/DNS；**61 个 API 端点**（原版 47 个全部覆盖，另多 14 个复刻端自用） |
| MCP | ✅ | `vshell-server mcp`：13 个工具，JWT 鉴权（与面板同一套，401 自动重登）；可驱动 http 与 tcp 客户端 |
| 协议互通 | ⚠️ | 帧/认证/任务记录逐字节对齐；官方 agent 认证+conf 通过，`main` 偶发成功（[GAPS A-1](docs/GAPS.md)） |
| 交付 | ✅ | `~/vshell-release`：服务端 + 前端 + 通用 agent（`-server/-vkey/-salt`）+ 启停脚本 |

### TCP 传输能力矩阵（首选传输）

| 能力 | 状态 | 证据 |
|---|---|---|
| 上线（checkin） | ✅ | e2e §7v |
| 任务下发（只下发一次） | ✅ | `GetAndMarkPendingTasks`，日志 `poll client=2 -> 1 task(s)` |
| 命令执行 | ✅ | `echo tcp-alive` → `{"output":"tcp-alive"}` |
| 文件列表 | ✅ | `/`=26、`/tmp`=1198、`/etc`=314（面板与浏览器都验过） |
| 文件树导航 | ✅ | 浏览器打开 TCP 客户端的文件页：树 17 节点，点 `/etc` 得 314 行 |
| 上传（面板 → 目标机） | ✅ | `/api/file/upload` → 目标机 `/tmp/tcp-up.txt` 落盘 28B，列表可见 |
| 下载（目标机 → 浏览器） | ✅ | `/api/file/downloadtobrowser` 取 `/etc/hostname` → 5B `kali` |
| 插件运行 | ✅ | `sysinfo.elf`（1.6MB，base64 后 2.2MB）→ 返回 `host=kali os=linux/amd64 …`；帧上限已对齐到 32MB |
| 隧道端口转发 | ✅ | e2e §7v `tcp tunnel ok` |
| 断线自动重连 | ✅ | 服务端重启后 TCP agent 自动回线（`Reset()` + 重新 checkin） |

**测试**：服务端 194 个测试函数 + agent 78 个 + e2e 150 处断言 + ui-check（12 页渲染 + 13 条点击级 + 46 条静态断言）；
`./scripts/release-check.sh` 一条命令跑完 9 项门禁。

**按需求不做**（空占位或已删除）：通知（企业微信/钉钉）、OSS 上传、屏幕监控、屏幕截图、许可证；
Windows/macOS 专有功能（当前环境 Linux）。

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

前端构建（新前端在 `web2/`，服务端优先托管 `web2/dist`）：

```bash
cd web2 && pnpm install && pnpm build && cd ..
```

Agent（独立 module；也支持运行时参数，无需重新编译）：

```bash
cd agent && go build -o agent .
# 用法一：命令行参数（盐必须与监听器一致，空盐写 -salt ""）
./agent -server http://127.0.0.1:8082 -vkey vk_demo -salt ""
# 用法二：tcp 传输
./agent -server tcp://127.0.0.1:8084 -vkey vk_demo -salt ""
# 用法三：从面板「客户端生成」下载的载荷，配置已内嵌，直接运行即可
```

快速开始的已知限制（监听器端口、上线失败等）见 [docs/DEV.md §8](docs/DEV.md)。

## 常用命令

```bash
./scripts/release-check.sh     # 发版门禁（构建/vet/测试/竞态/agent/前端/API 对齐）
./scripts/e2e.sh               # 端到端：HTTP/KCP/WS/DNS/TCP + 面板契约 + 载荷
./scripts/ui-check.sh          # 前端 12 页渲染 + 12 条点击级流程
./scripts/mcp-smoke.sh         # 原生 MCP：13 工具 + dashboard + run_command
./scripts/make-templates.sh    # 生成载荷模板（下载功能的前置条件）
```

## 目录结构

```
.
├── main.go          # 服务端入口
├── router/          # HTTP/WS 路由
├── controllers/     # 面板 API（beego 控制器）
├── c2engine/        # C2 引擎：监听器、协议、载荷、存储、流中继
├── utils/           # 配置、鉴权、许可证（可删）、通知
├── agent/           # Agent（独立 Go module）
├── web2/            # 新前端（Ant Design Vue + UnoCSS + Tailwind，默认由 /app/ 托管）
├── web/             # 上一版前端（Element Plus，保留作对照）
├── static/          # 原版 4.9.3 SPA 资产（前端文案/图标取证的来源）
├── views/           # 遗留模板（死代码，待删）
└── docs/            # ← 事实来源
```

## 路线图一句话

当前处于「按 4.9.3 复刻」阶段：后端（TCP 为首选传输）+ MCP + 新前端均已可用；
剩余主要是前端细节对齐与官方 agent `main` 通道。逐轮变更见 [docs/ROADMAP.md](docs/ROADMAP.md)。

## 许可

本仓库不含原版许可；原版软件的一切权利归其作者所有。**仅供研究，禁止商用或非法用途。**
