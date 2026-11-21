# 架构（ARCHITECTURE）

> 本文描述**当前代码**的结构与数据流（不是原版结构，也不是目标结构）。
> 目标结构与差距见 [ROADMAP.md](ROADMAP.md)、[STATUS.md](STATUS.md)。

---

## 1. 进程与部署形态

```
┌─────────────────────────┐        ┌──────────────────────────┐
│  vshell-server (main)   │        │  agent (独立 go module)  │
│  ├─ Web 面板 (beego)    │◀──WS──▶│  ├─ transport (http/kcp/ │
│  ├─ C2 engine (内存+SQL)│◀─HTTP─▶│  │   tcp/dns)             │
│  └─ 监听器 (http/kcp/   │◀─KCP──▶│  ├─ dispatcher (43 opcode)│
│      dns/ws)            │◀─DNS──▶│  ├─ 终端/屏幕/文件/持久化│
└─────────────────────────┘        └──────────────────────────┘
        │ SQLite db/data.db                 │ ldflags 注入配置
        │ conf/setting.conf                 │ (ServerAddr/VerifyKey/EncryptSalt)
        ▼                                   ▼
    面板浏览器 (static/)                目标主机
```

- **两个 Go module**：根 module `vshell`（服务端 + 面板 + 引擎），`agent/`（独立 module）。
  二者**没有任何共享包**，协议靠约定 —— 这也是历史上两端漂移没人发现的根因（见 STATUS §4）。
- 服务端单进程：Web 面板与 C2 监听器同进程，靠不同端口区分。
- agent 单进程：配置在编译期用 `-ldflags -X` 注入（目标：改为解析自身尾部配置块，见 B2.2）。

---

## 2. 服务端启动顺序（`main.go`）

```
loadConfig()                      // utils.GetFullSettings()，失败即 exit
  ↓ master_type=service ? webBanner : consoleBanner
controllers.AppStart(cfg)          // engine.Init + 注册 listenerManager
  ↓
router.InitRouter()                // net/http mux + beego handler + WS 路由
  ↓ go http.ListenAndServe(...)
controllers.AppBackground()        // 在线巡检/健康检查/告警/授权分支
  ↓ select{}                       // 无优雅关闭路径
```

`AppStart` 内做了什么（`controllers/engine.go`）：
1. `c2engine.GetEngine().Init(cfg)`：打开 `db/data.db`，建 4 张表，回填 clients/listeners/hosts/tasks，
   重建 verifyKey 索引。
2. 注册 listener 管理器，供面板 `listener/start` 调用。

`AppBackground`：授权分支（可删，见 ROADMAP 附注）、在线巡检、通知（钉钉/企微）。

---

## 3. HTTP 路由分层（`router/router.go`）

```
mux (net/http)
├── "/"                → 静态文件 + SPA 回退（跳过 /api、/c2）
├── "/api/getUserInfo" → 直接 handler（不走 beego）
├── "/api/getMenuList" → 直接 handler
├── "/api/getPermCode" → 直接 handler
├── "/api/"            → beego.BeeApp.Handlers
│      └── ApiBaseController.Prepare() 统一鉴权（Token 头 / token 查询串）
├── "/api/terminal/ws" → 先校验 token，再 Hijack 升级（beego 无法承载）
├── "/api/screen/ws"   → 同上
├── "/c2/"             → 恒 404（原版无 HTTP C2 端点）
├── "/index6us"        → 面板入口（原版注册的路径）
└── "/admin"           → 自写面板（与后端契约不符，见 STATUS）
```

**为什么 WS 不走 beego**：handler 会 Hijack 连接，beego 在方法返回后仍会尝试渲染视图 →
`Unknown view path:views` 并接管已劫持的响应。注释在 `router.go:105-119`。

---

## 4. 核心对象（`c2engine/types.go`）

| 对象 | 关键字段 | 说明 |
|---|---|---|
| `Client` | `ID/VerifyKey/Addr/Status/IsConnect/LocalIP/UserName/HostName/OsName/ProcessName/Arch` | 一个被控端；`Status` **当前无人写**（STATUS §4-③） |
| `Listener` | `ID/Mode/ListenAddr/ConnectAddr/VerifyKey/EncryptSalt/PingInterval` | 监听器；`Mode ∈ {http,https,dns,websocket,cdn_websocket,reverse}` |
| `OfficialTCPServer` | `official_tcp.go` | **官方协议** tcp 服务端：帧层复用 `FrameEncrypt/FrameDecrypt`，按抓包实现 认证→通道注册→任务/结果（真官方 agent 可连上并上报 client；见 RE-NOTES E-33…E-47） |
| `OfficialAgentSim` | `official_agent_sim.go` | 官方 agent 模拟器：无需原版载荷即可验证上述服务端（e2e §7p） |
| `Tunnel` | `ID/ClientID/Port/Mode/Target/RunStatus` | 隧道；数据面由 `c2engine.TunnelServer` 承载 |
| `Host` | `ID/ClientID/Host/Target/Scheme` | 反连主机（无会话处理） |
| `Task` | `ID/ClientID/Command/Timeout/Status/Result` | 命令；**持久化进 tunnels 表**（有反编译依据） |
| `Flow` | 流量计数 | 每客户端一份 |

`Engine` 是全局单例（`GetEngine()`），所有 map 操作走 `sync.RWMutex`。

---

## 5. 一条命令的完整路径（现状）

```
面板 POST /api/terminal/shell {id, command}
  → ApiBaseController.Prepare（校验 JWT）
  → TerminalController.Shell
      → 检查 client.Status  ← ★ 当前恒 false，直接返回 "client is close"
      → Engine.CreateTask(clientID, "terminal_start ..." / "id", timeout)
      → 等待结果 (waitTaskResult 轮询 Task.Status)
  ↓                          ↑
监听器会话从引擎拉任务          结果回写
  ├─ HTTP:  GET /api/tasks?client_id&verify_key   → TaskResponse(JSON)
  ├─ KCP:   LinkMsgMain {"type":"task_poll"}      → JSON
  └─ DNS:   TXT 查询                               → 单条任务
  ↓
agent: transport → executeCommand(字符串)
  ├─ "terminal_*" 前缀 → 终端会话
  ├─ "screen_capture*" 前缀 → 屏幕
  ├─ 首字节命中 opcode 表 → dispatchNativeCommand（43 槽）
  ├─ JSON {"type":...} → 文件/系统信息/持久化
  └─ 其它 → 交给 /bin/sh 或 cmd.exe
  ↓
agent → SendResult(taskID, 文本结果)
  → HTTP POST /api/result（base64+帧）/ KCP LinkMsgMain JSON
  → listener.handlePostResult → Engine.UpdateTask / forwardStreamingResult
```

**问题**：面板下发的文本命令（`ls`、`cat`、`screenshot quality=N`、`install`、`download`）
agent 侧没有对应解析器，全部落进 shell 报错 → 面板功能看起来"点了没反应"。
M1 的做法是把这条路径改成 opcode 记录（B1.1/B1.3）。

---

## 6. 代码地图

| 路径 | 职责 | 关键文件 |
|---|---|---|
| `main.go` | 启动 | — |
| `router/` | 路由 + 几个直接 handler | `router.go` |
| `controllers/` | 面板业务（beego 控制器） | `engine.go`（引擎门面）、`base.go`（鉴权/JSON 助手）、`file.go`、`terminal.go`、`screen.go`、`download.go` |
| `c2engine/` | C2 引擎 | `engine.go`（注册表）、`listener.go`（HTTP/WS）、`kcp.go`（KCP + Link）、`dns.go`、`protocol.go`（帧/加解密）、`wire.go`（24B 记录）、`channel_frame.go`（通道帧）、`payload*.go`（载荷）、`storage.go`（SQL）、`stream_hub.go`（终端/屏幕中继）、`health.go`/`monitor.go` |
| `utils/` | 配置/鉴权/日志/通知/许可证 | `config.go`、`auth.go`、`license.go`（可删）、`notify.go` |
| `agent/` | 被控端（独立 module） | `main.go`（3.2k 行，含 43 opcode 分派）、`message_wire.go`（帧加密）、`message_crypto.go`、`transport_real.go`（池化 TCP）、`dns_transport.go` |
| `static/` | 前端资产 | `index.html` + `assets/`（原版 SPA，200 chunk）、`admin/index.html`（自写，契约不符） |
| `views/` | 遗留模板 | `dashboard.html` + `static/js/app.js`（死代码，建议删除） |

---

## 7. 存储

- SQLite 文件：`db/data.db`（可用 `Config.DBPath` 覆盖）。
- 表：`clients` / `tunnels` / `hosts` / `listeners`（schema 见 `storage.go:28-122`）。
- **tasks 复用 tunnels 表**：`Id/ClientId/Status/Ports(=Command)/Remark(=Result)`。
  依据：原版 `LoadTaskFromSqlFile` 就是 `select * from tunnels`。改动前先确认这条依据仍成立。
- **tunnels 持久化在 JSON 侧车**（`db/tunnels.json`）：tasks 与 tunnels 共用 SQLite 的
  tunnels 表（沿用原版做法），无法区分两类行，因此隧道单独存 JSON（有意偏差）。

---

## 8. 并发与超时

| 位置 | 机制 |
|---|---|
| Engine map | `sync.RWMutex`，读多写少 |
| 监听器 | 每连接一个 goroutine；`stopCh` 关闭 |
| KCP | `communicationLoop` + 心跳 ticker(10s) + `ReadFlagRetry` |
| 结果等待 | `waitTaskResult` 轮询任务状态（`controllers/stream_wait.go`），超时 `shellWaitTimeout` |
| 流式中继 | `stream_hub` 按 (kind, clientID) 存一个 viewer；第二个观众会顶掉第一个 |

---

## 9. 已知架构问题（不改会持续踩）

1. **agent 与引擎无共享契约**：两个 module 各写一份 `CheckinRequest/ResultRequest`，已经漂移（STATUS §4-④）。
   → 目标：抽出 `protocol/` 共享包（可作为独立 module 或用 `go.work`）。
2. **三套前端并存**（原版 SPA / 自写 admin / 遗留 views），契约各不相同。→ M6 统一重写。
3. **命令层两套词汇**（文本 + opcode）。→ M1 收敛为一套。
4. **监听器管理器三份**（`C2Listener`、`KCPListener`、`ListenerManager`、`StartListenerByMode`），
   入口分散、部分零调用。→ M1 收敛为 `ListenerManager` 单一入口。
