# 开发指南（DEV）

---

## 1. 环境

| 工具 | 版本 | 备注 |
|---|---|---|
| Go | ≥ 1.25（本机 1.27 亦可） | `go.mod` 声明 1.25.7 |
| Node | ≥ 20 + pnpm | 仅前端（M6） |
| SQLite | 内嵌 `modernc.org/sqlite` | 无需安装 |

---

## 2. 构建

```bash
# 服务端（Web 面板 + C2 引擎）
go build -o vshell-server .

# agent（独立 module）
cd agent && go build -o agent .
cd ..
```

交叉编译 agent（示例：Windows 目标）：

```bash
cd agent
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "\
  -X main.ServerAddr=http://1.2.3.4:8443 \
  -X main.VerifyKey=vk_demo \
  -X main.EncryptSalt=salt_demo" -o agent.exe .
```

> agent 配置当前靠 `-ldflags -X`，与"面板生成载荷"的目标形态不同（见 ROADMAP B2.2）。

---

## 3. 运行

```bash
mkdir -p conf
cat > conf/setting.conf <<'EOF'
master_type=web
web_title=VShell
web_port=8082
web_ip=0.0.0.0
web_basic_auth=false
web_jwt_secret=dev-secret
web_username=admin
web_password=admin123
log_level=7
EOF
./vshell-server
```

访问 `http://127.0.0.1:8082/`（原版 SPA）或 `/admin`（自写面板，契约不全）。

**注意**
- 配置项全集见 `utils/config.go:applyConfigKey`。
- 不写 `conf/setting.conf` 也能起，但会生成随机密码（打印在日志）。
- 监听器默认绑 `0.0.0.0:443`（因为 `listener/add` 不解析 Host），**非 root 会启动失败** ——
  B0.10 修好之前，测 KCP/HTTP 请用 root 或直接调用 `c2engine.NewListener` 指定端口。
- beego 会打印 `open conf/app.conf: no such file or directory`，是它的默认配置探测，可忽略
  （或用 `utils/config.go` 里的路径生成 `conf/app.conf` 消除）。

---

## 4. 测试

```bash
go test ./...                 # 服务端 5 包
cd agent && go test ./...      # agent 13 组
go vet ./...
```

**测试现状与盲区**：单测全绿，但单测覆盖不到"服务端 ↔ agent"两个 module 之间的协议漂移
（历史上 7 处断点就是这么漏掉的）。跨模块的底线由 `scripts/e2e.sh` 兜住，CI 里跑：

```bash
./scripts/e2e.sh       # 约 60 秒：HTTP/KCP/WebSocket/DNS 四条通道 + 面板契约 + 下载 + 批量删除
./scripts/e2e.sh -v    # 额外打印命令回显
```

四条通道各跑一条 `id` 并校验回显；任一条断掉都会整体失败并打印两端的日志尾巴。

新增能力时，先在 `scripts/e2e.sh` 里加断言，再写实现。

### 4.1 端到端脚本（已落地，见 `scripts/e2e.sh`）

覆盖步骤：

```
1. go build 服务端 + agent；临时目录里起服务（static/ 软链接，不污染仓库）
2. 登录拿 token
3. POST /api/listener/add {Mode:"http", Vkey, Host:"127.0.0.1", Port:<随机>} → start
4. 用监听器的 Salt/Vkey 编译 agent 并启动
5. 轮询 /api/client/list 直到 clientOnlineCount > 0
6. POST /api/terminal/shell {id, command:"id"}，断言 output 含 uid=
7. 断言 /client/list 的字段契约（Id/OsName/.../IsConnect）
8. GET /api/download/stage 断言 200 + 非空 body
9. 批量删除（JSON 数组）断言生效
10. KCP 通道（Mode=kcp，agent ServerAddr=kcp://…）
11. WebSocket 通道（Mode=websocket，agent ServerAddr=ws://…）
12. DNS 通道（Mode=dns + DNSDomain，agent ServerAddr=dns://…、DNSServer=127.0.0.1:<port>）
```

---

## 5. 调试

| 场景 | 方法 |
|---|---|
| 看引擎状态 | 启动日志 `[C2Engine] Engine initialized with ...` |
| 看 checkin | `[C2Engine] New client: user@host (ID: n)` |
| 看任务下发 | `[C2Engine] New task ...` / 监听器日志 |
| 看面板 401 | `ApiBaseController.Prepare`（`controllers/base.go:64`） |
| 看协议帧 | `tcpdump -i lo -X 'tcp port 18080'`；帧头是 `u32 LE` 长度 |
| 解帧 | `go test ./c2engine -run TestFrameCapturedShape -v` 有现成的解帧用例 |
| SQLite | `sqlite3 db/data.db '.tables'`、`select * from clients;`（tasks 在 tunnels 表） |
| agent 侧 | agent 日志带 PID 前缀；`log.Printf("Task %d: %s")` |

---

## 6. 新增一条命令的完整流程（模板）

以"加一个 `netstat` 面板命令"为例：

1. **协议**：确认载体 —— 走 opcode（原版）还是文本（过渡期）。优先 opcode。
2. `docs/PROTOCOL.md` 登记记录格式（opcode 号、argv 布局）。
3. **服务端**：`c2engine` 里构造记录 → `Engine.CreateTask` → 监听器取走下发。
4. **agent**：`agent/main.go` 的 `decodeNativeCommand` 命中 → `dispatchNativeCommand` 加 case →
   `emitFrames` 产出结果记录。
5. **结果**：服务端 `ParseAgentFrame` 解析 → 写 Task.Result（或流式转发）。
6. **面板**：`controllers/*` 暴露 HTTP 接口 → `docs/API.md` 登记。
7. **测试**：单元测试 + `scripts/e2e.sh` 增加断言。
8. **文档**：更新 [STATUS.md](STATUS.md) 对应行。

---

## 7. 代码规范

- 注释里出现原版函数地址（如 `FUN_010952e0`）时，必须能对应到 [RE-NOTES.md](RE-NOTES.md) 的证据编号。
- 未恢复的值一律 `// PLACEHOLDER — <原因>`，并登记进 [STATUS.md](STATUS.md) §5。
- 不要新增"看起来能用"的辅助函数而不接线路：当前仓库已有 20+ 死代码（STATUS §6）。
- 中文注释 + 英文标识符；错误信息用英文（日志/HTTP body 保持原样风格）。

---

## 8. 常见坑

| 现象 | 原因 | 处理 |
|---|---|---|
| 面板所有操作返回 `client is close` | 客户端未被标记在线 | R1 已修（checkin 置在线 + 60s 巡检）；排查时看 `/client/list` 的 `IsConnect` |
| agent 上线失败、服务端日志只有 400 | body 编码不匹配 | R1 已修（服务端容错解码）；仍失败就抓包看帧头 |
| 监听器总是 443 | 面板没传地址 | R1 已修（支持 `Host/Port/ListenAddr`，缺省按协议给默认端口） |
| 下载按钮 404 | 路由只有 POST | R1 已修（GET+POST 双注册） |
| 客户端表格空白 | 字段大小写不符 | R1 已修（输出大写契约字段） |
| KCP agent 连上但命令没回显 | 帧头未剥/键名不一致 | R1 已修（`decryptWire` + `hsString`）；见 `docs/PROTOCOL.md` §2.2 |
| `go build ./...` 在 agent 目录留下 `agent` 二进制 | Go 的默认行为 | 用 `go build -o /tmp/agent .` |
