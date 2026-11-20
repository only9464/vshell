# 逆向证据登记（RE-NOTES）

> 用途：给代码里每一处"与原版对齐"的断言一个可查的出处。
> 规则：**代码注释里引用 FUN 地址或结论时，必须指向本文的证据编号。**
>
> 证据分级：
> - **硬证据**：抓包、实测响应、gdb 寄存器、可复现的黑盒行为
> - **推断**：由多条硬证据收敛得到（需写明收敛方式）
> - **未恢复**：明确承认不知道，不得在代码里假装知道

---

## 已恢复（硬证据）

| 编号 | 结论 | 出处 | 代码位置 |
|---|---|---|---|
| E-01 | 传输帧 `<u32 LE len><[12B nonce][ct][16B tag]>`，AES-256-GCM | 抓包解帧成功；`c2engine` 有 captured-frame 测试 | `c2engine/protocol.go:198-247`、`agent/message_wire.go` |
| E-02 | GCM 参数 nonce 12 / 偏移 0 / 无 AAD / tag 16 | 对捕获帧穷举（nonce 8–20 × offset 0–4 × AAD∈{无,nonce} × tag 8–20）唯一解 | 同上 |
| E-03 | 帧密钥 = `hex(md5(salt))` 的 32 个 ASCII 字节（AES-256） | gdb 抓到 RDX 为 32 字节 ASCII；`md5("salt")` 与该值一致；原始 16 字节摘要与零填充均不通过 tag 校验 | `FrameSaltKey` |
| E-04 | 任务分派 `FUN_010952e0`：跳表 43 项，索引 = `opcode - 1`，0x05/0x19 指向 default，故 41 个有效 opcode | 反汇编 `DEC R9; CMP R9,0x2a; JA default; JMP [0x1dc31e40+R9*8]`（0x1095828） | `agent/main.go:879-922` |
| E-05 | 任务参数：`argv[0]`=opcode，其后 4 字节**大端** int32 | `FUN_00fc1620` | `agent/main.go:2472-2476` |
| E-06 | 结果记录 24 字节：`kind u8 / flags u8 / u16 / u32 / u32 / u32 / u64` | `FUN_0100d160`（写缓冲 +0x88/+0x90/+0x94） | `c2engine/wire.go`、`agent/main.go:1078-1118` |
| E-07 | 通道帧 `[type u8][id u32 LE][data]`，上限 0xff5，数据帧最小 0xd | `FUN_011b4040/011b5600/011b5b40/011b6ba0/011b7020`，6 个测试 | `c2engine/channel_frame.go` |
| E-08 | DNS 信道编码 = Go 标准 base32、**无填充** | 黑盒：原版报错 `NXDOMAIN: base32 decoding: illegal base32 data` | `c2engine/dns.go:161-176` |
| E-09 | SQL schema 四张表 clients/tunnels/hosts/listeners；**tasks 存在 tunnels 表** | `FUN_011aef60` 建表语句；`LoadTaskFromSqlFile` = `select * from tunnels` | `c2engine/storage.go:28-122,365-393` |
| E-10 | checkin = 文本命令头 + JSON：`conf`→NewClient、`host`→NewHost、`stus`→状态、`task`→任务 | `FUN_016f3e80`，KCP/WS 通道实测 | `c2engine/kcp.go:530-580` |
| E-11 | 原版无 HTTP C2 端点（`/c2/l/*` 全 404） | 黑盒扫描 | `router/router.go:234-241` |
| E-12 | 面板路由 = `/index6us`（11 字节，从数组解密） | `FUN_0193b5e0` | `router/router.go:175-177` |
| E-13 | po 字符串链（家族 A）可逐字节还原：`"stageless/ebpf_%s_%s"`、`"windows_amd64.exe"` | 置换链 `plaintext = dst - src (mod 256)`；`po_decode.go` 有回归向量 | `c2engine/po_decode.go:182-214` |
| E-14 | garble 字符串池解码 = `dst_blob - src_blob`（逐字节 mod 256），loader `0x116d020`、decode 站 `0x116d0c2` | 反汇编 | `c2engine/string_decrypt.go` |
| E-15 | SPA 线契约：`/runner/runplugin` 参数 `pluginName`/`procArg`；屏幕 `quality ∈ {big,normal,small}`；截图返回裸 base64；`/download/*` 返回裸二进制 + `Content-Disposition` | 前后端联调 + 前端 chunk 内字面量 | `docs/API.md` |
| E-16 | 登录 = 明口令比对 → JWT；JWT claims `{token_type,user_id,role_id,username,exp,iat}` | 黑盒登录响应逐字 | `utils/auth.go:192-208` |
| E-17 | SQLite 运行时 schema 与 `decodedCreateTables` **逐列一致**（clients 22 / tunnels 25 / hosts 23 / listeners 15） | 直接 dump 本机原版 `db/data.db`（R-12） | `c2engine/storage.go:28-122` |
| E-18 | 面板成功/错误文案全集（add/save/start/stop/delete/modified success、connection error、client is close、already start、not found …） | 黑盒逐接口探测（R-01/R-06） | `controllers/*`、`tests/contract_test.go` |
| E-19 | 载荷模板路径 `stageless/<tp>_<arch>`、`dll/<tp>_<arch>`、DNS 前缀 `ebpf_` | 黑盒 download 错误信息（R-07） | `c2engine/payload_build.go:PoTemplatePath` |
| E-20 | `listener/add` 即运行态、`EncryptSalt` 缺省空、`MaxDNSsize=512`；list 项 14 键 | 黑盒（R-02/R-03/R-04） | `controllers/listener.go` |
| E-21 | 空列表序列化为 `items: null` | 黑盒（R-05） | `controllers/{listener,tunnel}.go` |
| E-22 | `/api/getMenuList`、`/api/getPermCode` 在原版 404；`/api/client/add` 原版 500 | 黑盒（R-09/R-10） | `router/router.go`（保留为扩展，登记偏差） |

---

## 推断（写明收敛方式，可被推翻）

| 编号 | 结论 | 收敛方式 | 风险 |
|---|---|---|---|
| I-01 | 密钥派生用 `md5(salt)` 而非 HMAC/PBKDF2 | E-03 的 gdb 值与 md5 碰撞一致；其它候选（sha256、md5 原始字节）均失败 | 低 |
| I-02 | 41 个 opcode 的语义名 | 由分派 case 分支读到的字段偏移命名（如 `+0x60` → interval） | 中：名称是描述性的，不是原版字符串 |
| I-03 | tasks 复用 tunnels 表时各列映射（`Ports`=Command、`Remark`=Result） | 由写入/读取两侧的列顺序推断 | 中：需真实 DB 样本佐证 |
| I-04 | 屏幕帧 zlib 压缩 | 前端 `pako.inflate` 必需；原版压缩级别未知 | 低（级别不影响互通） |
| I-05 | `quality → (q,fps)` 映射 90/10、60/5、30/2 | **无依据**，按大小关系编造 | 高（见占位清单） |

---

## 未恢复（不得假装知道）

| 编号 | 项 | 现状 | 影响 |
|---|---|---|---|
| U-01 | argv entry 的 2 字节 key | 只知框架 `[2B][0x3D][data]`，值由调用方闭包提供 | `PoArgvEntry` 恒失败（B2.3） |
| U-02 | `PoArgvSeed`（0x2a 是占位） | per-closure 种子在 `FUN_01914a40/01914b40/01914c40` 内部 | 同上 |
| U-03 | 每个 opcode 的字符串常量表 | 曾在 `0x1e302680` 的 67 项表上误判（实为 SQLite pragma 名）；881 个可解池中无 VShell 字符串 | opcode 文本无法对齐（B3.1） |
| U-04 | 屏幕 quality → 采集参数 | 未定位换算函数 | 画质档位不可信（B3.4） |
| U-05 | `FUN_017019c0` 状态机字符串 | 深加密，静态放弃 | 部分面板文案无法还原 |
| U-06 | `FUN_011a02e0` / `01564720` 解密 stub | 静态求解受阻 | 相关分支无法复现 |
| U-07 | 内嵌 agent 二进制 | Go 的 `{ptr,len}` 记录由 `init` 通过 `loaduintptr` 填充，静态不可提取 | 载荷必须自带模板（B2.1） |
| U-08 | `FUN_0194d140` 后台服务 | 仅 license/`licTime` 路径对齐 | 后台任务不完整 |
| U-09 | 结果记录的**线缆形态** | 只恢复了缓冲内布局，没有任何函数把这 24 字节直接发出 | B1.2 需要额外证据 |

---

## 更正记录（曾经写错、已推翻的结论）

> 这些结论历史上写进过 README/代码，现全部作废。**不要重新引入。**

| # | 错误结论 | 事实 | 影响面 |
|---|---|---|---|
| X-01 | 消息加密是 `0x458f00` 的 XOR 密钥流/双块 CBC 链 | `0x458f00` 是 **garble 字符串/字段名解密**（rdx=0x8e2240 选串表），与消息加密无关 | `agent/message_crypto.go` 头注释已改 |
| X-02 | 存在常量 `msgFrameKey`（写死在源码里） | 该常量是早期猜测，**在原版二进制里不存在**；密钥是运行期由盐派生 | 已删除 |
| X-03 | 消息帧 = `[16B IV][21B ct]` = 37B | 真实帧是 `<u32 LE len><[12B nonce][ct][16B tag]>` | 文档/代码已改 |
| X-04 | 服务端 AES-128 `FUN_0053a1e0` 是消息解密 | 只验证了 roundtrip，与消息帧无关 | 不能作为协议依据 |
| X-05 | 41 个 opcode 中 0x04/0x08/0x18 是死分支，实际 39 个 | 之前把反编译 `case K` 当成 opcode 字节；`case K` 实为 opcode `K+1`，三者都有处理器 | 已纠正（E-04） |
| X-06 | `0x1e302680` 的 67 项表是"命令名表" | 是 SQLite 内嵌驱动的 `sqlite3Pragma` 名表，运行时由 `0x117b989` 重写 | U-03 重新开放 |
| X-07 | 登录密码需要先做摘要再比对 | 配置值**原文**比对（`web_password`），前端传明文 | `utils/auth.go` |
| X-08 | 面板 API 是 REST 风格 `/api/clients` | 原版是 beego 风格 `/api/client/list`（POST） | 曾据此写了错误的前端/自写面板 |

---

## 参考环境（活体）

本机有原版 4.9.3 实例与 IDA（见 [REFERENCE.md](REFERENCE.md)）。黑盒取证脚本
`scripts/probe-original.sh` 把响应抓成 `testdata/original/*.json`，并由
`controllers/contract_test.go` 逐项比对——这是"行为等价"的回归底线。

## 待办：如何补证据

| 目标 | 方法 |
|---|---|
| U-01/U-02 | 对 `FUN_018dca00` 的调用方下断点，读闭包捕获的栈槽（gdb，x64 调参） |
| U-03 | 运行期发各个 opcode，抓结果帧的 kind/字段，反推名称与语义 |
| U-04 | 对比同一画面在三个 quality 下的 `screen_frame` 尺寸/字节数/间隔 |
| U-09 | 抓真实下发瞬间的链路字节（KCP Main 帧内层），确认是 24B 记录还是别的封装 |
