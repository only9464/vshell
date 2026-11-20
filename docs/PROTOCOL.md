# 线协议（PROTOCOL）

> 本文分两部分：**P1 现状协议**（代码里实际在跑的）与 **P2 目标协议**（与原版对齐的方向）。
> 实现改动必须先改本文，再改代码。

---

## 0. 术语

| 术语 | 含义 |
|---|---|
| 传输帧 (frame) | `<u32 LE len><12B nonce><ciphertext><16B tag>`，AES-256-GCM |
| 链路帧 (link frame) | KCP/TCP 上的 `[type u8][len u16 BE][payload]` |
| 记录 (record) | 24 字节类型化字段，原版任务/结果的载体 |
| 通道帧 (channel frame) | `[type u8][id u32 LE][payload]`，隧道/端口转发用 |
| 监听器 (listener) | 服务端一组端口与协议配置，含 VerifyKey 与 EncryptSalt |

---

# P1 · 现状协议

## 1. 传输帧（加密层）

```
wire  = u32le(len) || frame
frame = nonce[12] || ct || tag[16]          // AES-256-GCM，无 AAD
key   = hex(md5(salt)) 的 32 个 ASCII 字节  // 即 AES-256 的 32 字节密钥
```

- `salt` = 监听器的 `EncryptSalt`（面板字段「流量加密盐」）。
- 证据：抓包帧可解；GCM 参数由穷举唯一确定（nonce 12/offset 0/无 AAD/tag 16）。
  见 [RE-NOTES.md](RE-NOTES.md) E-03。
- 实现：服务端 `c2engine/protocol.go:191-247`；agent `agent/message_wire.go`。
- **当前缺陷**：agent 侧 `msgFrameSalt` 硬编码为 `"salt"`（`agent/message_wire.go:123`），
  且 HTTP 通道把帧再 base64 了一层，服务端不解 base64 → 见 [STATUS.md](STATUS.md) §4-①②。

## 2. 通道封装

### 2.1 HTTP / HTTPS

```
POST {listener}/api/checkin     body = frame(JSON CheckinRequest)
GET  {listener}/api/tasks?client_id=N&verify_key=K   → JSON TaskResponse
POST {listener}/api/result      body = frame(JSON ResultRequest)
GET  {listener}/{swt,sww,swk,sws,swd,swl,swld}       → 载荷二进制
GET  {listener}/health                                → 健康检查
```

`CheckinRequest`（两端字段名已一致，均为 snake_case）：

| 字段 | 类型 | 说明 |
|---|---|---|
| `verify_key` | string | 监听器密钥 |
| `hostname` / `username` / `os` / `process` | string | 目标机信息 |
| `local_ip` / `arch` / `pid` / `version` | string/int | 目标机信息 |

`ResultRequest` —— **两端不一致（STATUS §4-④）**：

| 服务端（`c2engine/protocol.go:135-141`） | agent（`agent/main.go:595-603`） |
|---|---|
| `ClientID` / `CommandID` / `Result` / `Status` / `VerifyKey` | `client_id` / `command_id` / `result` / `status` / `verify_key` |

> 修复方案（B0.5）：服务端用自定义 `UnmarshalJSON` 同时接受两种拼写，并保留大写键输出以兼容原版。

### 2.2 KCP（`c2engine/kcp.go`）

- KCP over UDP，AES block crypt 由 `(salt+key)[:32]` 派生（`kcp.go:470`），FEC 10/3。
- 握手：**裸 JSON**（无长度前缀、无类型字节），服务端逐字节读到 JSON 解析成功为止（`kcp.go:542-552`）。
- 之后：`[type u8][len u16 BE][payload]`（`kcp.go:141-158, 282-287`）。

| type | 名称 | 方向 | 载荷 |
|---|---|---|---|
| 0x01 | Main | 双向 | JSON（任务轮询/结果/心跳） |
| 0x02 | Config | 服务端→agent | JSON 配置 |
| 0x03 | Chan | 双向 | 隧道数据（**当前只打日志**） |
| 0x04 | Health | 服务端→agent | 健康信息 JSON |
| 0x05 | Close | 双向 | 空 |

**当前缺陷**：agent 先 GCM 再写链路帧，服务端按明文 JSON 解析；握手键名 `VerifyKey` vs `verify_key`。
见 STATUS §4-⑥。

### 2.3 DNS（`c2engine/dns.go`）

```
查询名 = <base32(消息)>.…[.<agentID>].<Domain>
消息   = "checkin:host|user|os" | "task" | "result:<taskID>:<data>"
响应   = TXT 记录，内容 "ok:<clientID>:<interval>" / "task:<id>:<base32(cmd)>" / "noop" / "ack"
```

- base32：Go 标准字母表、**无填充**（黑盒实锤：原版报错 `base32 decoding: illegal base32 data`）。
- **当前缺陷**：agent 侧用 `rgst:`/`main:`/`conf:` 前缀且不读 TXT 响应；服务端绑死 `:53`（失败退 5353），
  忽略监听器地址。两端等于各写一半。

## 3. 20 字节/24 字节记录（原版任务与结果载体）

服务端已有编码器（`c2engine/wire.go`），agent 已有生成器（`agent/main.go:1078-1160`），
但**尚未接到线路上**（sink 恒 nil）。布局：

```
+0x00  kind   u8
+0x01  flags  u8
+0x02  u16
+0x04  a      u32
+0x08  b      u32
+0x0c  c      u32
+0x10  d      u64
```

常用帧码（节选，全集见 `wire.go:30-46`）：`0x54` 列表行、`0xa6` ack、`0xb1/0xb2` pong/ping、
`0x9b` dump、`0x48` 标量结果、`0x12` 会话行。

任务记录（下发方向）：`[opcode u8][argv…]`，数值参数为 4 字节**大端** int32（`FUN_00fc1620`）。

## 4. 通道帧（隧道，`c2engine/channel_frame.go`）

```
type u8 || id u32le || data[]
type: 0 结束 / 1 会话数据1 / 2 会话数据2 / 3 会话命令1 / 4 会话命令2
      5 会话(8B 数据字段) / 6 ACK / 7 关闭 / 8 通道数据
上限 0xff5 (4085)；数据帧最小 0xd
```

已实现并测试，但**零调用**（B1.6 接入）。

---

# P2 · 目标协议（对齐方向）

## 5. 三次收敛

| 收敛点 | 现在 | 目标 |
|---|---|---|
| 命令载体 | 面板发文本（`ls`/`cat`/`screenshot quality=N`） | 面板发**记录**（opcode+argv），文本仅保留给终端输入 |
| 结果载体 | 文本 + JSON | 记录序列（24B），服务端 `ParseAgentFrame` 解析 |
| 通道 | 4 套各自为政（http/kcp/dns/ws） | 统一"链路帧 + 记录"两层，通道只负责搬运 |

## 6. 目标分层

```
┌─────────────────────────────────────────┐
│ 语义层：记录（24B）/ opcode / 通道帧      │
├─────────────────────────────────────────┤
│ 安全层：传输帧（AES-256-GCM，密钥=md5盐）│
├─────────────────────────────────────────┤
│ 搬运层：HTTP body / KCP link / DNS TXT / WS binary │
└─────────────────────────────────────────┘
```

搬运层只做"把一段字节送到对端"，不解释内容；安全层只在搬运层之上加一次封解；
语义层由 `protocol/`（计划新增的共享包）统一定义。

## 7. 兼容性矩阵（当前）

| 通道 | 服务端 | agent | 互通 | 备注 |
|---|---|---|---|---|
| HTTP/HTTPS | ✅ | ✅ | ✅ | R1 打通（服务端容错解码 + 盐派生） |
| KCP | ✅ | ✅ | ✅ | R1 打通（握手键名容错 + 主信道帧加解密） |
| DNS | ⚠️ 单任务、无 ack | ⚠️ 不读响应 | ❌ | M1 处理 |
| WebSocket | ✅ | ❌ 无客户端 | ❌ | M1 处理 |
| 裸 TCP | ❌ 无监听器 | ✅ | ❌ | M1 处理（二选一） |

> 目标：M1 结束时 DNS 双向闭环并新增 `cmd/protocol-probe` 交叉验证。

---

## 8. 变更记录

| 日期 | 变更 |
|---|---|
| 2026-11-20 | 建立本文；记录 P1 现状与 P2 目标分层 |
