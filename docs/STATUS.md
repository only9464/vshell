# 还原状态（STATUS）

> 最后更新：R1（2026-11-20）。判定口径见文末「判定标准」。
> 任何"已实现"的条目都必须能被 `go test` 或 `scripts/e2e.sh` 复现，否则降级为"部分"。

---

## 1. 总览

| 维度 | 当前 | 说明 |
|---|---|---|
| 代码结构还原 | ~75% | 包/表/帧结构与原版对应，注释带反编译地址 |
| 面板功能可用性 | ~65% | 契约（字段/文案/空列表）已按黑盒逐项对齐；dashboard 改为真实采集；上传/插件仍为壳 |
| 端到端投递闭环 | ~85% | R1 打通 HTTP/KCP，R3 打通 WS/DNS —— **四条通道全部 e2e 通过**（§4） |
| 与官方二进制协议互通 | ~10% | 加密帧对齐；命令层是自造文本/JSON（M1 处理） |
| 面板 API 契约等价 | ~85% | `controllers/contract_test.go` 对 16 个端点做键集/文案比对（原版 4.9.3 抓包） |
| 载荷投递 | ~70% | 模板按原版路径加载 + 自描述配置块；面板生成的载荷零 ldflags 可上线（与原版格式对齐仍待 U-01/U-02） |

构建与测试基线（R1 实测）：`go build ./...` 通过；`go test ./...` 5 包全绿；
`cd agent && go test ./...` 通过；`scripts/e2e.sh` 通过（HTTP + KCP 双通道、面板契约、下载、批量删除）。
**测试全绿 ≠ 端到端可用** 曾长期成立，现在由 e2e 脚本兜住这条底线。

---

## 2. 模块矩阵

### 2.1 面板 / controllers

| 模块 | 状态 | 证据 / 缺口 |
|---|---|---|
| 登录 + JWT | ✅ 已实现 | `controllers/login.go:19-53`、`utils/auth.go:192-208`；响应 `{token,userInfo}` 与 SPA 的 `const {token}=a` 匹配 |
| 注销 | ⚠️ 形式 | JWT 无状态，`engineDelToken` 空实现 |
| 客户端列表 | ⚠️ 字段契约不符 | 后端出 `id/ip/os/arch`（`client.go:70-81`），SPA 读 `Id/OsName/UserName/...` |
| 客户端备注/删除文件/杀进程 | ✅ 下发通 | `client.go:116-149` → `dispatchCmd`；但受 B0.1 阻塞 |
| 监听器 CRUD/启停 | ⚠️ 半 | `listener.go` 全实现，但 `Add` 丢弃 Host → 恒绑 443（`engine.go:201`） |
| 隧道 CRUD/启停 | ✅ | start 起数据面、stop 关闭；增删即落盘 `db/tunnels.json`，重启可恢复 |
| 进程/持久化 | `delprocess` → `kill` JSON、`install/remove` → `persist/cleanup` JSON（e2e 验证进程真被杀掉） | `controllers/client.go`、`install.go` |
| 文件管理 11 个接口 | ✅ | `ls`/`cat`/`edit`/`upload`/`download`/`wget`/`touch`/`modifytime`/`mv`/`rm`/`getdisk` 全部走 JSON 命令并 e2e 验证；`downloadtooss` 取回暂存后明确报 OSS 未实现 |
| 文件上传 | ✅ | 读 multipart → base64 → 下发 `{"type":"upload"}`；e2e 用 `cat` 校验落盘 |
| 下载到浏览器 | ✅ | JSON 命令取 base64 → 附件输出（e2e 用 /etc/hostname 校验）；进度/OSS 仍为壳 |
| 载荷下载 6 接口 | ⚠️ 半 + 方法错配 | 生成逻辑存在，但路由仅 POST（`router.go:85-90`） |
| 插件列表/上传 | ✅ 真 | `runner.go:37-41,184-198` |
| 隧道数据面 | 服务器在隧道端口监听；公网连接到达→分配 chanID→下发 `tunnel_open`→agent 拨回（`VST1` + chanID + target）→配对后裸 TCP 双向转发；未配对 15s 超时关闭 | `c2engine/tunnel_server.go`、`agent/tunnel.go`、`c2engine/tunnel_server_test.go` |
| 插件执行 | ❌ 桩 | 恒返回 `runplugin payload not aligned`（`runner.go:112-126`） |
| 终端 shell / WS | ✅ 真 | `terminal.go`；受 B0.1 阻塞 |
| 屏幕 WS | ⚠️ 半 | 流中转真，quality 参数是编造值（`screen.go:95-105`） |
| 截图 | ✅ | 面板改发 `{"type":"screenshot"}` JSON；失败时把 agent 原文回给前端 |
| dashboard | ❌ 假数据 | `engine.go:296-340` 全硬编码 |
| 设置 | ✅ | 写回 `conf/setting.conf`（`utils.UpdateConfig`） |
| 鉴权 | ⚠️ 弱 | `getMenuList/getPermCode` 只判非空；RBAC 零调用；无 Basic Auth |

### 2.2 引擎 / c2engine

| 模块 | 状态 | 证据 / 缺口 |
|---|---|---|
| Storage（4 表 schema） | ✅ | `storage.go:28-122`；tasks 复用 tunnels 表（`storage.go:372`，有反编译依据） |
| Engine 注册表（client/listener/tunnel/host/task） | ✅ | `engine.go` |
| 在线状态 | ❌ | **无任何代码把 `Client.Status` 置 true** |
| HTTP/HTTPS 监听 | ⚠️ | 唯一真正闭环的通道，但两端 body 格式不一致（§4-②） |
| KCP 监听 | ⚠️ | 链路层真（FEC 10/3、`[1B type][2B BE len]`），上层协议两端不一致 |
| DNS 监听 | ⚠️ | 绑 `:53` 忽略 ListenAddr；只回一条任务；无 ack（`dns.go:181-240`） |
| WS/CDN-WS | ⚠️ | 服务端有，agent 无 WS 客户端 |
| 隧道数据面 | ✅ | 反向端口转发（`tunnel_server.go` + `agent/tunnel.go`），与 C2 传输解耦 |
| 通道帧 | ⚠️ | 已实现 + 6 测试，但零调用（`channel_frame.go`） |
| 载荷模板 | ❌ | 无 `go:embed`，`agents/` 不存在 → 恒 nil |
| 载荷修补 | ⚠️ | 仅尾部追加配置块；argv key 未恢复 → `PoArgvEntry` 必失败 |
| OSS 上传 | ⚠️ | `oss.go` 真发 HTTP；`protocol.go:538` 的 `UploadToOSS` 只打日志（死代码） |
| 健康检查 | ✅ | TCP/HTTP/ICMP 真；连续失败自动重启数据面（保留 OnOpen）；未配置目标不计成功/失败 |
| 监控 | ✅ | 删除了 `c2engine/monitor.go` 的假数据；面板指标统一由 `controllers/metrics.go` 读 /proc 采集 |
| 字符串解密/置换链 | ✅ | `po_decode.go`、`string_decrypt.go` 有回归向量，但未接运行时 |

### 2.3 Agent

| 能力 | 状态 | 说明 |
|---|---|---|
| shell 执行 | ✅ | 管道，非 PTY（`main.go:2731`） |
| 终端会话 | ✅ | Unix 走 PTY（真 TTY、resize 生效）；Windows 为管道回退 |
| 屏幕截图/直播 | ⚠️ | 截图覆盖 grim/import/scrot/gnome-screenshot/spectacle，失败信息透传；每帧仍拉一次子进程（待改流式抓屏） |
| 文件操作 | ✅ | 分块下载（1 MiB/块）+ 进度回填 + 上传落盘 |
| 进程/服务/网络枚举 | ⚠️ | 有枚举，但 opcode 分支只回计数文本 |
| 持久化安装/卸载 | ✅ | 注册表 Run + schtasks / crontab（`main.go:2846-2900`） |
| 隧道数据面 | 服务器在隧道端口监听；公网连接到达→分配 chanID→下发 `tunnel_open`→agent 拨回（`VST1` + chanID + target）→配对后裸 TCP 双向转发；未配对 15s 超时关闭 | `c2engine/tunnel_server.go`、`agent/tunnel.go`、`c2engine/tunnel_server_test.go` |
| 插件执行 | ✅ | 可执行插件（.exe/.elf）落盘执行并回显；动态库与 .net 明确未实现 |
| 41 opcode | ⚠️ | 真 22 / 部分 9 / 桩 10（见 §3） |
| 结果帧 | ❌ | sink 恒 nil，只发文本 |
| 传输 | ⚠️ | http(坏)/kcp(半)/tcp(无对端)/dns(不解析)/ws(无客户端) |
| 隐身/提权 | ❌ | 仅 `HideWindow` |

---

## 3. opcode 覆盖（41 个有效 + 2 个 default 槽）

| 分组 | opcode | 状态 |
|---|---|---|
| 真实现（22） | 0x01 0x03 0x04 0x05 0x06 0x07 0x08 0x0b 0x0d 0x0e 0x11 0x12 0x17 0x18 0x1b 0x19 0x1e 0x1f 0x21 0x23 0x24 0x27 0x28 0x29 0x2a | 读写状态/发标量帧 |
| 部分（9） | 0x0f 0x13 0x14 0x17 0x1a 0x1c 0x22 0x25 0x26 | 只回计数/固定文本，帧未对齐 |
| 桩（10） | 0x02 0x09 0x0a 0x0c 0x10 0x15 0x16 0x1d 0x20 0x2b | 返回 `not implemented` |

（注：0x17 在两表都出现是分类口径重叠，实际状态以代码为准：`netroute` 只做部分写入。）

---

## 4. 端到端断点清单

### 4.1 已在 R1 修复（有 e2e 断言）

| # | 原断点 | 修复 | 证据 |
|---|---|---|---|
| ① | agent 发 `base64(AES-GCM帧)`，服务端只解裸帧 → 400 | 服务端容错解码（JSON/裸帧/base64 帧/base64 JSON） | `c2engine/agent_body.go`、`TestDecodeAgentBody` |
| ② | agent 帧密钥写死 `"salt"` | `SetMsgFrameSalt(EncryptSalt)`；监听器自动生成盐 | `agent/message_wire.go`、e2e 断言 salt 非空 |
| ③ | `Client.Status` 永不置 true → 所有下发 `client is close` | checkin 置在线 + 60s 静默巡检下线 | `Engine.NewClient`/`SweepOffline`、`liveness_test.go` |
| ④ | 结果 JSON 键名不一致（`ClientID` vs `client_id`） | `ResultRequest.UnmarshalJSON` 双拼写 | `protocol.go`、`TestResultRequestAcceptsBothSpellings` |
| ⑤ | 下载端点只注册 POST，SPA 用 GET → 404 | 7 个端点 `get:...;post:...` | `router.go`、e2e 断言 200 |
| ⑥ | `client/list` 字段大小写与前端不符 → 表格空白 | 输出大写契约字段（保留旧键一版） | `controllers/client.go`、e2e 逐键断言 |
| ⑦ | `dellist` 不读 JSON 数组 → 恒 `id err` | `jsonBodyRaw` + 数组/字符串/逗号三形态 | `controllers/base.go`、e2e 断言删除生效 |
| ⑧ | `listener/add` 丢 Host → 恒绑 443 | 解析 `ListenAddr/Host/Port/Salt/PingInterval` | `controllers/listener.go`、e2e 自定义端口 |
| ⑨ | KCP 握手键名 + 加密方向不一致 → 配 key 即拒连 | `hsString`/`msgInt64`/`sendMain`/`openMain`；agent 侧 `decryptWire` 剥头 | `c2engine/kcp.go`、`agent/main.go`、e2e KCP 段 |

### 4.1b R2 修复（对齐原版 4.9.3 黑盒）

| 项 | 内容 | 证据 |
|---|---|---|
| 登录 | 改为原版信封 `{code,message,result{desc,realName,roles,token,userId,username},type}`；失败文案 `Incorrect account or password！` | `testdata/original/login.json` |
| 监听器 | `add` 即启动（`start` → `already start`）；成功文案 add/save/start/stop/delete success；list 14 键；空列表 null；盐缺省空 | R-02…R-05 |
| 隧道 | add 需 clientId（缺失 → `未找到客户端`）；start/stop 缺失 → `start error`/`stop error`；空列表 null | R-06 |
| 客户端 | editremark 缺失 → `client ID not found`；delfile 缺失 → `client id not found`；delprocess/dellist → `delete success` | R-06 |
| 文件/截图/安装/插件 | 缺失客户端 → `connection error`（terminal 例外，仍是 `client is close`） | R-06 |
| 设置 | get 返回三键；edit → `modified success` | R-06 |
| 仪表盘 | 39 键对齐；cpu/disk/load/io/tcp/udp/icmp 改为读 `/proc` 真实采集；web_port 取配置 | `controllers/metrics.go` |
| 载荷 | 模板路径改为 `stageless|dll/<tp>_<arch>`、DNS `ebpf_` 前缀；缺失模板报 `open …: file does not exist`；去掉 MinimalPE 外壳 | E-19 |
| 契约测试 | `controllers/contract_test.go`：16 个端点键集/文案与原版抓包比对 | `testdata/original/` |

### 4.1c R3 修复

| 项 | 内容 | 证据 |
|---|---|---|
| WebSocket 通道 | agent 新增 `wsTransport`（ws/wss，JSON 帧协议与 `c2engine.WSMessage` 对齐）；e2e 用 `Mode=websocket` 监听器 + 真 agent 跑通 `id` | `agent/transport_ws.go`、`scripts/e2e.sh` §12 |
| Basic Auth | 401 + `WWW-Authenticate: Basic realm="Authorization Required"` + body `401 Unauthorized\n`；只保护真实静态文件，`/api/*` 与 agent 路径不受保护 | `router/basicauth_test.go`、REFERENCE R-13/R-15 |
| 静态服务 | `/` → 302 `/index.html`；未知路径 404（去掉自造的 SPA 回退）；`/index.html` 直接 200（不 301 到 `/`） | `router/router.go`、REFERENCE R-14 |
| 测试稳定性 | router WS 测试的 SQLite 文件移出 `t.TempDir()`，消除 "directory not empty" 抖动 | `router/stream_ws_test.go` |
| DNS 通道 | 8B agentID 前缀、base32 标签分块（60 字符/块）、TXT 响应 base32 解码、结果 60 字节/片上传、FQDN 尾点修正、`task` 无冒号命令解析 | `c2engine/dns.go`、`agent/dns_transport.go`、`agent/dns_transport_test.go`（假 DNS 服务器 3 个用例） |

### 4.1d R5 修复（载荷链路）

| 项 | 内容 | 证据 |
|---|---|---|
| 面板生成载荷可上线 | 服务端在模板尾部追加 `VSCF` 自描述配置块（JSON：server/vkey/salt/mode[/dns_server]），agent 启动时从自身文件尾读回并覆盖构建期值 | `c2engine/config_block.go`、`agent/config_block.go`、`e2e §14` |
| 协议前缀 | 配置块里的 server 带 scheme（http/kcp/ws/https/dns），agent 据此选传输 | `poServerURL` |
| 模板缺失 | 直接报 `open <path>: file does not exist`（原版文案），不再生成 MinimalPE 假外壳 | `PoBuild` |
| 单测 | 配置块写入/读回/缺失/长度非法四个用例 | `c2engine/config_block_test.go` |
| 文件上传 | multipart → base64 → agent 落盘；e2e 上传后用 `cat` 校验 | `controllers/file.go:Upload`、`e2e §7b` |
| 设置持久化 | `/setting/edit` 写回 `conf/setting.conf`；`/setting/get` 读配置 | `controllers/setting.go` |
| 终端 PTY | Unix 用 creack/pty；`terminal_pty_test.go` 断言 `[ -t 1 ]` 为真且 `stty size` 跟随 resize | `agent/terminal_unix.go`、`terminal_pty_test.go` |
| 隧道数据面 | 服务器在隧道端口监听；公网连接到达→分配 chanID→下发 `tunnel_open`→agent 拨回（`VST1` + chanID + target）→配对后裸 TCP 双向转发；未配对 15s 超时关闭 | `c2engine/tunnel_server.go`、`agent/tunnel.go`、`c2engine/tunnel_server_test.go` |
| 插件执行 | 面板读文件下发；agent 落盘（0700）执行、60s 超时、清理临时目录；非法名/坏 base64/非零退出都有测试 | `agent/plugin_exec_test.go` |
| 屏幕输入 | `screen_input move/click/scroll/key` → 解析 → xdotool argv（`screen_input_test.go` 覆盖 4 类事件与非法输入）；e2e 断言命令确实路由到 agent | `agent/screen_input.go` |
| 通知 / OSS | 需要第三方凭证，按操作者要求**暂不实现**（字段保留、上传明确报未实现），见 `docs/GAPS.md` §F |
| 原生 MCP | `vshell-server mcp`：13 个工具覆盖客户端/命令/文件/隧道/截图/概览；stdio JSON-RPC；登录取 JWT 规避跨进程密钥不一致 | `mcp/`、`docs/MCP.md` |
| 巡检/维护 | `AppBackground` 现在启动健康巡检 + 维护任务（此前是死代码）；`Health` 懒初始化避免 panic | `controllers/engine.go`、`c2engine/health.go` |
| 分块下载/进度 | agent 支持 `offset/length` 并按块回 `chunk:<total>:<b64>`；面板 1 MiB/块拉取并回填进度；e2e 用 3 MiB 文件校验字节数与 `pre:100` | `agent/main.go`、`controllers/file.go` |

**有意偏差**：配置块格式是复刻端自定义的（原版块与 argv key 未恢复，见 U-01/U-02），
原版 agent 不认这个块；等逆向补齐后应改为原版格式。

### 4.2 仍存在的断点（M1/M2 处理）

| # | 断点 | 影响 | 计划 |
|---|---|---|---|
| ⑩ | 面板下发的是文本命令（`ls`/`cat`/`screenshot quality=N`），agent 只认 `terminal_*`/`screen_capture*`/JSON/opcode | 文件管理、截图、安装等功能点下去无反应 | B1.1/B1.3 |
| ⑪ | 原生 opcode 的结果帧没有上线（`resultFrameSink` 恒 nil） | 面板拿不到结构化结果 | B1.2 |
| ⑫ | 原版 agent 无法被本服务端配置（原版配置块/argv key 未恢复，U-01/U-02） | 与官方 agent 互通仍不通；自研 agent 已闭环 | B2.3 |
| ⑬ | ~~DNS 两端各半~~ | R3 已闭环（见下） | ✅ B1.4 |
| ⑭ | 隧道数据面缺失（`LinkMsgChan` 只打日志） | 隧道建了不能转发 | B1.6/B4.1 |

## 5. 占位清单（`PLACEHOLDER` / 编造值，必须逐项替换或删除）

| 位置 | 占位内容 | 计划 |
|---|---|---|
| `c2engine/payload_build.go:239` | `PoArgvSeed = 0x2a` | B2.3 逆向 |
| `c2engine/payload_assets.go:93` | argv entry 2 字节 key 为空 | B2.3 逆向 |
| `controllers/screen.go:95-105` | 画质 90/10、60/5、30/2 | B3.4 反推 |
| `c2engine/stream_hub.go:272-284` | zlib 级别 | B3.4 |
| `c2engine/payload.go:266-282` | `MinimalPE` 无 stub 代码 | B2.4 |
| `c2engine/monitor.go` | 磁盘/TCP/UDP 常量 | B4.5 |
| `c2engine/protocol.go:538` | `UploadToOSS` 只打日志 | B5.8 或删除 |
| `agent/main.go:1120` | 结果帧未上线 | B1.2 |

---

## 6. 死代码清单（写了但零调用，容易误判为已实现）

`EncodeShellCommand/EncodeFileListCommand/EncodeFileUploadCommand/EncodeFileDownloadCommand/EncodeScreenshotCommand`
（整组自造命令编码）、`BuildAgentDownloadCommand/BuildLinuxAgentDownloadCommand/BuildPowershellDownloadCommand`、
`GenerateClientToken`、`ReadMagic/WriteMagic`、`UploadToOSS`、`DecompressGzipPayload`、
`c2engine/buffered_io.go`（整文件）、`c2engine/net_compat.go`（整文件）、
`CreateTunnelOnLink`、`HandleAgentSession`、`NewCDNWebSocketListener`、`StartListenerByMode`、
`utils.EnforceClientLimit`、`utils/auth.go` 的 RBAC 组、`c2engine/monitor.go:Collect`。

处理策略：先跑 `deadcode`/`staticcheck` 确认，M0 结束时统一清理（保留有测试引用的用例文件）。

---

## 7. 判定标准

- **已实现 ✅**：有实现 + 有测试或 e2e 证据 + 与文档描述一致。
- **部分 ⚠️**：主路径可跑，但字段/协议/边界与原版不同，或只覆盖一种输入。
- **桩/缺失 ❌**：返回固定字符串、只打日志、或根本没有实现。
- 任何"对齐"表述必须能指向 [RE-NOTES.md](RE-NOTES.md) 里的证据编号；纯推断一律写"推断"。
