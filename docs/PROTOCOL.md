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

### 2.3 WebSocket（`c2engine/listener.go` + `agent/transport_ws.go`）

服务端 `/ws`（CDN 回源走 `/cdn/ws`），JSON 文本帧，字段与 `c2engine.WSMessage` 一致：

```jsonc
→ {"type":"checkin","data":{...CheckinRequest(snake_case)...}}
← {"type":"checkin","client_id":1,"session_id":"sess_...","status":"ok","data":{"interval":5,"timeout":30}}
→ {"type":"poll","client_id":1}
← {"type":"poll","data":{"tasks":[{"id":1,"command":"...","timeout":30}],"interval":5}}
→ {"type":"result","data":{"command_id":1,"result":"...","status":"completed"}}
→ {"type":"ping"}   ← {"type":"pong"}
```

> R3 之前 agent 侧没有 WS 客户端（互通矩阵里是 ❌）；现在两端都有，e2e 覆盖。
> 注意：这是**复刻端自造**的 JSON 帧协议，原版的 WS 与 KCP 共用"链路帧 + 记录"两层，M1 之后收敛。

### 2.4 DNS（`c2engine/dns.go` + `agent/dns_transport.go`）

对称实现（R3 打通，e2e 覆盖）：

```
查询名 = <labels>.<zone>            labels = base32([8B agentID 大端][消息]) 按 60 字符切块
响应   = TXT 记录（服务端把响应 base32 后按 60 字符切多条 TXT，agent 拼回再解码）
```

| 消息 | 方向 | 载荷 | 响应 |
|---|---|---|---|
| `checkin:<host>\|<user>\|<os>` | agent→server | — | `ok:<clientID>:<interval>` |
| `task` | agent→server | — | `task:<taskID>:<base32(cmd)>` 或 `noop` |
| `result:<taskID>:<seq>/<total>:<b64>` | agent→server | 60 字节原始数据/片 | `ack` |
| `result:<taskID>:<data>` | 兼容 | 短结果整体 | `ack` |

要点（**踩过的坑，别再踩**）：
1. 域名总长 ≤255、单标签 ≤63 —— 长消息必须切块；结果上传按 60 字节/片，
   片大小是量出来的（96 字节时整名 268 字节，服务端根本收不到查询）。
2. 问题名是 FQDN（带尾点），域名后缀比较前必须 `TrimSuffix(".")`。
3. 服务端 TXT 响应是 base32 编码的，agent 必须先解码。
4. `task` 这种无冒号命令以前被解析成 `"unknown"`，导致 DNS 通道永远取不到任务。
5. 服务端 task 分支改用 `GetAndMarkPendingTasks`（原子领取），剩余任务放回 pending。

### 2.5 载荷尾部配置块（复刻端扩展）

```
payload = <模板字节> <blob> <blobLen u32 LE> "VSCF"
blob    = JSON {"server","vkey","salt","mode"[,"dns_server","proxy"]}
```

- `server` 带 scheme（`http://`/`kcp://`/`ws://`/`https://`/`dns://`），agent 据此选传输；
- agent 启动时读自身可执行文件末尾 8 字节定位块，覆盖 `-ldflags` 注入值；
- 与 PE/ELF 结构无关（追加在文件尾，加载器不会执行到）；
- **有意偏差**：原版配置块与 argv 加密 key 尚未恢复（U-01/U-02），此块仅自研 agent 识别。

### 2.5b 屏幕控制命令（复刻端文本词汇）

| 命令 | 语义 | agent 实现 |
|---|---|---|
| `screen_capture_start quality=N fps=M` | 开始推流 | 定时抓屏 → `screen_frame:<fmt>:<idx>:<q>:<b64>` |
| `screen_capture_stop` / `_quality=N` / `_fps=N` | 停止/调参 | — |
| `screen_input move x=.. y=..` | 鼠标移动 | xdotool mousemove |
| `screen_input click button=1..3` | 点击 | xdotool click |
| `screen_input scroll dir=up|down amount=N` | 滚轮 | xdotool click --repeat N 4/5 |
| `screen_input key=<KEYSYM>` | 按键/组合键 | xdotool key --clearmodifiers |

> 注意：`screen_input` 与 `screen_capture*` 一样，必须在 `executeCommand` 里先于
> shell 分支被路由，否则会落进 `/bin/sh` 执行（历史 bug，R7 修复）。

### 2.5c 插件命令（复刻端 JSON 词汇）

```jsonc
→ {"type":"plugin","name":"fscan.x64.elf","arg":"-h 10.0.0.0/24","data":"<base64 插件字节>"}
← "<插件 stdout+stderr>"        // 失败时同时给出 "plugin exited: ..." / 超时
```

- 面板从 `plugins/<name>` 读字节（`/runner/upload` 会落到该目录）；
- agent 把字节写到临时目录（0700）后 exec，60s 超时，结束清理；
- 动态库（`.dll/.so/.dylib`）与 `.net` 托管程序集**明确返回未实现**，不静默降级；
- 原版是把插件字节按十六进制拼进 `runplugin <hex> <procArg> <bool>` 文本命令，
  复刻端改为 JSON 下发（登记为偏差）。

### 2.5d 官方载荷配置区（R13 动态取证，已复刻）

模板内预留 **20480 字节 `0x19` 占位区**，下发时原地写入：

```
region = [16B IV][AES-128-CBC(JSON + 0x00 填充)]     // key = IV = md5(vkey)
明文     = {"server":"host:port","type":<tp>,"vkey":...,"proxy":...,"salt":...,
            "l":true,"e":false,"d":30,"h":10}
```

- `type` ∈ `tcp` / `kcp` / `ws` / `dns` / `oss`（**官方无 `http`**）
- 无 PKCS#7，明文固定 20464 字节，靠 `0x00` 填充
- 复刻端 `c2engine/agentconfig.go` 的编码器与官方产物**逐字节一致**
  （黄金向量：`testdata/original/payload/config_region_*.bin`）
- 复刻端仍额外追加 VSCF trailer 供自家 agent 读取（有意偏差，见 API.md §12）
- `proxy` 官方恒为空（R14 取证：`proxy`/`Proxy`/`proxyUrl` 三种写法都不进配置区）
- 官方产物里**没有 argv 条目**（R14 取证：stageless 产物与模板只差配置区）

### 2.6 文件传输命令（复刻端 JSON 词汇）

| 命令 | 请求 | 响应 |
|---|---|---|
| 上传 | `{"type":"upload","path":"<dir>/<name>","data":"<b64>"}` | `uploaded: <path> (<n> bytes)` |
| 下载（整文件） | `{"type":"download","path":...}` | 裸 base64 |
| 下载（分块） | `{"type":"download","path":...,"offset":N,"length":M}` | `chunk:<total>:<b64>` |

面板按 1 MiB 分块拉取，逐块回填 `downloadTasks` 进度，供 `/file/getdownloadper` 计算百分比。

## 2.7 官方 tcp 传输的真实记录（R20 MITM 实证）

```
线格式：[u32 LE 长度][12B nonce][AES-256-GCM 密文][16B tag]
密钥  ：key = hex(md5(salt))（32 字节 ASCII；salt 为空 → d41d8cd9…）
记录  ：[u32 LE 长度] + 文本/JSON
```

握手（明文）：`"4.9.3"` ×2 → 服务端 32 字节十六进制挑战 → agent 32 字节应答 →
`"sucs"` → `"conf"` → `\x01` → `"conf \x01\x00\x00{client json}"` → `\x01` →
16 字节会话 ID → `"main"`。心跳：`00 ff ff ff ff 1e 00` + RFC3339 时间戳，
服务端回 `08 ff ff ff ff 1e 00`。详见 `docs/RE-NOTES.md` E-34。

## 2.8 官方面板命令 → agent 任务记录（R21 实证）

信封：`[u32 LE 长度][type u8][channel u32 LE][payload]`，
type = `0x06` 服务端 ACK / `0x03` 数据 / `0x05` 进度 / `0x07` 结束 / `0x01` 客户端 ACK。

任务 JSON：

```jsonc
{"ConnType":"v","Host":"v","LocalProxy":false,"RemoteAddr":"v",
 "Req":{"Pass":"v","Type":"M","File":null,"Z1":"id","Z2":"","Z3":"","Z4":"","Z5":""},
 "Option":{"Timeout":5000000000}}
```

类型码：`M` 执行命令、`B` 列目录、`C` 读文件、`A` 磁盘、`D` touch、`J` mkdir、
`I` 移动（`src|<-dst/`）、`DSELF` 删除、`Upload`（`File`=base64，Z1=路径）。
结果：`->|<输出>|<-`。

复刻端结构见 `c2engine/official_task.go`（字段顺序与抓包一致，黄金向量测试）。

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

## 3.5 隧道数据面（反向端口转发）

```
公网客户端 ──TCP──▶ 服务器 :Port ──(下发 tunnel_open)──▶ agent ──TCP──▶ 目标
                    ▲                                      │
                    └────────── agent 拨回 :Port ──────────┘
握手 = "VST1" | chanID u32 LE | targetLen u16 LE | target
```

1. 服务器在隧道端口监听，公网连接到达 → 分配 chanID → 把它挂到 pending；
2. 通过任务队列给 agent 下发 `{"type":"tunnel_open","id":chanID,"addr":"<服务器:端口>","target":"<目标>"}`；
3. agent 带 `VST1` 握手拨回同一端口，服务器据 magic 区分 agent 与公网连接；
4. 配对后双向 `io.Copy`；15s 内未配对则关闭公网连接。

> 为什么不用 `LinkMsgChan`：原版隧道走 C2 长连接的通道帧，而本复刻端四条传输都是
> 请求/响应式，搬过去会变成"每条数据等一个轮询周期"。反向转发与 C2 传输解耦，
> 延迟接近裸 TCP，且 HTTP/KCP/WS/DNS 四种 agent 都能用（只要能出网到隧道端口）。
> `channel_frame.go` 的编解码保留作对齐参考（当前无调用）。

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
| DNS | ✅ | ✅ | ✅ | R3 打通（分片结果上传 + TXT base32 解码） |
| WebSocket | ✅ | ✅ | ✅ | R3 打通（自造 JSON 帧；待 M1 收敛为记录层） |
| 裸 TCP | ❌ 无监听器 | ✅ | ❌ | M1 处理（二选一） |

> 目标：M1 结束时 DNS 双向闭环并新增 `cmd/protocol-probe` 交叉验证。

---

## 8. 变更记录

| 日期 | 变更 |
|---|---|
| 2026-11-20 | 建立本文；记录 P1 现状与 P2 目标分层 |
