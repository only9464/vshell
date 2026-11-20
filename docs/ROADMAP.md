# 复刻路线图（ROADMAP）

> 状态标记：`[ ]` 未开始 · `[~]` 进行中 · `[x]` 完成 · `[!]` 阻塞（需继续逆向）
> 工作量单位：人日（1 名熟悉 Go、可对照二进制的工程师）
> 更新纪律：每次提交（或每轮开发）同步勾选状态并在文末"变更记录"追加一行。

---

## 0. 完成定义（Definition of Done）

"后端完全复刻"按四条同时满足来判定，缺一不算完成：

| # | 判据 | 验证方式 |
|---|---|---|
| D1 | **端到端可用**：面板建监听器 → 生成载荷 → 目标机上线 → 终端/文件/屏幕/插件全部可用 | `scripts/e2e.sh` 全绿 |
| D2 | **协议自洽**：按 [PROTOCOL.md](PROTOCOL.md) 独立实现的第三方客户端能上线并执行命令 | `cmd/protocol-probe` 通过 |
| D3 | **行为等价**：面板 API 的请求/响应/错误码与原版黑盒一致（含字段大小写） | `docs/API.md` 契约测试 |
| D4 | **无占位**：`grep -rn "PLACEHOLDER\|not implemented" --include=*.go` 只剩"已证伪记录"类注释 | CI 检查 |

---

## 1. 里程碑总览

| 里程碑 | 主题 | 目标产出 | 预估 | 状态 |
|---|---|---|---|---|
| **M0** | 打通端到端 | 一条脚本跑完 `起服务→建监听器→编 agent→上线→回显` | 3–5 人日 | `[x]` |
| **M1** | 协议层对齐 | 24B 记录帧上下行、DNS/KCP/WS 三通道真闭环 | 10–15 人日 | `[ ]` |
| **M2** | 载荷链路 | 面板生成的二进制**不改一字**即可上线 | 10–20 人日 | `[ ]` |
| **M3** | Agent 能力 | 41 个 opcode 全实现；PTY、屏幕控制、插件执行 | 20–30 人日 | `[ ]` |
| **M4** | 引擎数据面 | 隧道转发、反连主机、真实监控、结果存储 | 15–25 人日 | `[ ]` |
| **M5** | 面板闭环 | dashboard/上传/下载/插件/设置/RBAC 全部真数据 | 10–15 人日 | `[ ]` |
| **M6** | 前端重写 | Vue3 面板（见 [FRONTEND.md](FRONTEND.md)） | 15–25 人日 | `[ ]` |
| **M7** | Windows 实战层 | UAC、注入/迁移、反沙箱、代理 | 20–40 人日 | `[ ]` |

**关键路径**：M0 → M1 → M2（这三块不完成，其余都是空中楼阁）。M3/M4 可与 M5/M6 并行。

---

## 2. M0 · 打通端到端（`[x]` 完成 · R1）

**验收（DoD）**：`scripts/e2e.sh` 从零开始，在无 root 的普通用户下 60 秒内完成
"起服务 → 登录 → 建监听器（自定义端口）→ 编译 agent → agent 上线 → 终端执行 `id` 并拿到回显"。

> **R1 结果**：已达成。`scripts/e2e.sh` 同时覆盖 HTTP 与 KCP 两条链路、`/client/list` 字段契约、
> `GET /api/download/stage`、`delList` 的 JSON 数组参数。耗时约 25 秒（含两次 `go build`）。

| ID | 任务 | 位置 | 验收 | 状态 |
|---|---|---|---|---|
| B0.1 | `Client.Status` 生命周期：checkin 置在线、心跳超期置离线 | `c2engine/engine.go`、`c2engine/health.go` | 上线后 `/api/client/list` 返回 `status:true`；`terminal/shell` 不再回 `client is close` | `[x]` |
| B0.2 | HTTP body 兼容：服务端在 JSON 解析失败后先试 base64 再解帧 | `c2engine/listener.go:handleCheckin/handlePostResult` | agent 出厂状态即可 checkin（不再 400） | `[x]` |
| B0.3 | 帧密钥接盐：agent 的 `msgFrameSalt` 由 `EncryptSalt` 派生，去掉硬编码 `"salt"` | `agent/message_wire.go:123`、`agent/main.go` | 监听器盐 ≠ `"salt"` 时仍能互通 | `[x]` |
| B0.4 | 监听器盐自动生成并落库 | `c2engine/engine.go:NewListener`、`controllers/engine.go:engineAddListener` | 新建监听器 `Salt` 非空且两端一致 | `[x]` |
| B0.5 | 结果通道键名统一：服务端同时接受 `CommandID/client_id` 与 `command_id/ClientID` | `c2engine/protocol.go`、`listener.go`、`kcp.go` | agent 回传结果能落到对应 task | `[x]` |
| B0.6 | KCP 握手键名兼容（`VerifyKey` vs `verify_key`）；KCP 载荷加密方向统一 | `c2engine/kcp.go:560`、`agent/main.go:kcpTransport` | KCP 监听器配了 key 时 agent 能上线 | `[x]` |
| B0.7 | `/api/download/*` 与 `/api/file/downloadtobrowser` 同时注册 GET | `router/router.go:81,85-90` | 浏览器直接访问返回 200 + 文件流 | `[x]` |
| B0.8 | `client/list` 响应按 SPA 契约出字段 | `controllers/client.go:70-81` | `Id/OsName/UserName/HostName/LocalIP/Addr/Tp/Location/ProcessName/IsConnect/Remark/PingCheckTime` 齐全 | `[x]` |
| B0.9 | 批量删除读 JSON body（`JsonGetIntList/JsonGetStrList`） | `controllers/base.go:166-196` | `dellist` 传 `{"id":[1,2]}` 生效 | `[x]` |
| B0.10 | `listener/add` 接受 `Host/Port`（当前 `Host` 被丢弃 → 恒绑 443） | `controllers/listener.go:59`、`controllers/engine.go:201` | 面板填 18080 就监听 18080 | `[x]` |
| B0.11 | 端到端脚本 | `scripts/e2e.sh`（新增） | CI/本地可一键验证 | `[x]` |

---

## 3. M1 · 协议层对齐

**验收**：面板下发的命令以 **24 字节记录**（opcode + argv）进入隧道；agent 的结果同样以记录序
列回传；`cmd/protocol-probe`（照 [PROTOCOL.md](PROTOCOL.md) 独立实现）能复现该链路。

| ID | 任务 | 位置 | 验收 | 状态 |
|---|---|---|---|---|
| B1.1 | 任务下发改为 `[opcode u8][argv…]` 记录（含 4B 大端整数参数） | `c2engine`（新 `task_wire.go`）、`controllers/*` | agent `decodeNativeCommand` 直接命中 | `[ ]` |
| B1.2 | 结果帧上线：`emitFrames` → 线路（当前 `resultFrameSink` 恒 nil） | `agent/main.go:1120`、`agent/transport_real.go` | 结果帧字节可被服务端 `ParseAgentFrame` 解出 | `[ ]` |
| B1.3 | 服务端命令词汇收敛：只发 opcode，删掉 `ls/cat/screenshot quality=N` 一类自造文本命令 | `controllers/file.go`、`screenshot.go` | 无 `dispatchCmd("ls …")` 残留 | `[ ]` |
| B1.4 | DNS 双向闭环：服务端 ack + 多任务；agent 解析 TXT 响应 | `c2engine/dns.go:181-240`、`agent/dns_transport.go` | DNS 模式跑通 e2e | `[ ]` |
| B1.5 | WebSocket 客户端（或补裸 TCP 监听器） | `agent/`、`c2engine/listener.go` | 任选一条链路 e2e 通过 | `[ ]` |
| B1.6 | 通道帧接入隧道数据面 | `c2engine/channel_frame.go`（已写但零调用） | 隧道里能传 TCP | `[ ]` |

---

## 4. M2 · 载荷链路

**验收**：面板点一次"生成"，下载得到的 exe 放进干净 Windows，**不做任何二次处理**就能上线。

| ID | 任务 | 位置 | 验收 | 状态 |
|---|---|---|---|---|
| B2.1 | 预编译模板 `go:embed`（平台×架构×模式矩阵）或服务端交叉编译 | 新 `c2engine/templates/`、`builder.go:137` | 六种 download 模式都产出可执行文件 | `[ ]` |
| B2.2 | agent 自解析尾部配置块（取代 ldflags） | `agent/main.go`（新 `config_block.go`） | 同一份 agent 换配置即换监听器 | `[ ]` |
| B2.3 | 恢复 argv entry 2 字节 key 与 per-closure 种子 | `c2engine/payload_assets.go:93`、`payload_build.go:239` | `PoArgvEntryReady()==true` | `[!]` |
| B2.4 | `MinimalPE` 补真实 stub（当前 entry 指向数据，不可执行） | `c2engine/builder.go:471` | 生成的 PE 能被 Windows 加载 | `[ ]` |
| B2.5 | shellcode/dll/listen/listendll 各自产物 | `c2engine/payload_build.go` | 四种模式产物类型正确 | `[ ]` |

---

## 5. M3 · Agent 能力

| ID | 任务 | 说明 | 状态 |
|---|---|---|---|
| B3.1 | 10 个桩 opcode：0x02/0x09/0x0a/0x0c/0x10/0x15/0x16/0x1d/0x20/0x2b | 前置：恢复原版运行期连接池/代理表/端口映射表布局 | `[ ]` |
| B3.2 | 9 个半实现收口：0x0f/0x13/0x14/0x17/0x1a/0x1c/0x22/0x25/0x26 | 现在只回计数文本，要发真记录帧 | `[ ]` |
| B3.3 | PTY / ConPTY | 交互、resize、echo | `[ ]` |
| B3.4 | 屏幕流 quality→参数映射 + 鼠标键盘控制帧 | 服务端已在发 `{"type":"1".."10"}`，agent 无处理器 | `[ ]` |
| B3.5 | 插件执行：落盘 → `.exe/.dll/.net` 分派 → 加载 | 两端当前都拒绝 | `[ ]` |
| B3.6 | 文件传输分块 + 进度回填 | 面板进度条现在是 0 | `[ ]` |

---

## 6. M4 · 引擎数据面

| ID | 任务 | 位置 | 状态 |
|---|---|---|---|
| B4.1 | 隧道数据面（`LinkMsgChan` + 通道帧 + 端口转发） | `kcp.go:1082`、`channel_frame.go` | `[ ]` |
| B4.2 | 反连 Host 会话（`AuthenticateHost` 现在恒 true） | `c2engine/app.go:530` | `[ ]` |
| B4.3 | tunnels/listeners 持久化补齐（现在 tunnels 重启全丢） | `c2engine/engine.go:909`、`storage.go` | `[ ]` |
| B4.4 | 健康检查：`tunnel.Health` 未初始化导致 panic；失败要重启 | `c2engine/health.go:125` | `[ ]` |
| B4.5 | 真实监控采集（monitor.go 全是常量） | `c2engine/monitor.go` | `[ ]` |

---

## 7. M5 · 面板闭环

| ID | 任务 | 位置 | 状态 |
|---|---|---|---|
| B5.1 | dashboard 真实数据（现在 40+ 键硬编码） | `controllers/engine.go:296-340` | `[ ]` |
| B5.2 | `/api/file/upload` 真收 multipart 并下发 | `controllers/file.go:205-228` | `[ ]` |
| B5.3 | `downloadtobrowser` 输出附件；`getdownloadper` 回填进度 | `controllers/file.go:282-306` | `[ ]` |
| B5.4 | `runplugin` 真实调用（现在恒 "not aligned"） | `controllers/runner.go:112-126` | `[ ]` |
| B5.5 | `/api/setting/*` 持久化到 `conf/setting.conf` | `controllers/setting.go` | `[ ]` |
| B5.6 | RBAC/权限码落地（`auth.go` 里已有，零调用） | `utils/auth.go:92-151` | `[ ]` |
| B5.7 | Basic Auth 中间件（配置项存在但无实现） | `router/router.go` | `[ ]` |
| B5.8 | OSS 上传真发请求（`UploadToOSS` 现在只打日志） | `c2engine/protocol.go:538` | `[ ]` |

---

## 8. M6 · 前端重写（Vue3）

详见 [FRONTEND.md](FRONTEND.md)。要点：以 [API.md](API.md) 为契约，不再反推原版 SPA。
原版 `static/assets` 仅作交互参考，不参与构建。

| ID | 任务 | 状态 |
|---|---|---|
| B6.1 | 脚手架（Vite + Vue3 + TS + Pinia + Vue Router + Element Plus） | `[ ]` |
| B6.2 | 登录/鉴权/动态菜单（`/api/login`、`getUserInfo`、`getMenuList`、`getPermCode`） | `[ ]` |
| B6.3 | 客户端列表/详情/备注/删除文件与进程 | `[ ]` |
| B6.4 | 监听器管理（增删改启停） | `[ ]` |
| B6.5 | 载荷下载页（六种模式，流式下载 + token） | `[ ]` |
| B6.6 | 终端（xterm.js + `/api/terminal/ws`） | `[ ]` |
| B6.7 | 屏幕直播（`/api/screen/ws`）与截图 | `[ ]` |
| B6.8 | 文件管理（树、编辑器、上传、下载、wget、OSS） | `[ ]` |
| B6.9 | 隧道 / 插件 / 设置 / 关于 | `[ ]` |

---

## 9. 风险与阻塞

| 风险 | 影响 | 应对 |
|---|---|---|
| `FUN_018dca00` 的 argv key / per-closure 种子未恢复 | B2.3 阻塞，载荷与原版 agent 不兼容 | 继续逆向；先用自有协议跑通（B2.2），保持占位可替换 |
| opcode 名表（0x43 项）未定位 | B3.1/B3.2 的文本与类别编号无法对齐 | 用运行期黑盒对照（发 opcode 看响应）反推 |
| 屏幕 quality→参数映射未恢复 | 画质档位是猜的 | 同上游对比码率/尺寸反推 |
| serT 状态机字符串深加密 | 部分分支无法静态解 | 动态调试（gdb/lldb 断点）优先 |
| 无原版服务端可对照 | 行为等价难验证 | 以抓包 + 面板黑盒响应为准，逐条登记到 API.md |

---

## 10. 变更记录

| 日期 | 轮次 | 变更 |
|---|---|---|
| 2026-11-20 | R1 | 建立文档体系（docs/）；**M0 完成**：端到端（HTTP+KCP）打通、面板字段/参数契约修正、e2e 脚本落地 |
