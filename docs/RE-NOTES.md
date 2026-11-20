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

## 原版 4.9.3 Linux 构建（本机活体）的静态恢复

| 编号 | 结论 | 证据 | 用途 |
|---|---|---|---|
| E-23 | `.gopclntab` 完好，但头部魔数被 garble 改为非标准值 `0x840c4ea6`；按字段宽度解析可恢复 **38645 个函数**（11699 个有名字） | `scripts/go-symbols.py`，产物 `docs/inventory/vshell-4.9.3-linux-syms.txt` | 所有"按地址定位函数"的工作 |
| E-24 | **类型名与方法名保留**（包名被随机化）：13 个业务控制器共 239 个方法可枚举到地址与字节数 | `docs/inventory/controllers.md` | 契约核对、定点反汇编 |
| E-25 | ~~字符串被加密~~ **更正：字面量是明文，紧凑打包**。`DelFile` 的 2 字节参数名就是 `id`（`lea 0x1b27b94` + `mov ecx,2`，该地址起 16 字节是 `idifinipivjskbki…`，即 `id` 后面紧跟别的短串）。之前读 16 字节造成"密文"的误判。已知明文示例：`stageless/`（46 处）、`ebpf_`（12）、`runplugin`、`checkin` | 直接 `bytes.find` + 精确长度读取 | 静态可读；但 46 处多数落在外层二进制的内嵌模板里 |
| E-26 | 控制器参数名可按"字面量长度"辅助判断（如 `DelFile` 先用长度 2 再长度 5 → `id`/`paths`），与我们的实现一致 | `ClientController.DelFile` @`0x1822d80` | 参数契约核对 |

### 常用地址（Linux 4.9.3）

| 函数 | 地址 | 字节数 |
|---|---|---|
| `ClientController.List` | `0x1820e00` | 6912 |
| `ClientController.DelFile` | `0x1822d80` | 448 |
| `ListenerController.Add` | `0x1833920` | — |
| `ListenerController.List` | `0x18310a0` | — |
| `DownloadController.Stage` | `0x1823960` | 3552 |
| `DownloadController.Listen` | `0x18259a0` | 4576 |
| `DownloadController.Dll` | `0x1829400` | 4544 |

> 完整清单见 `docs/inventory/controllers.md`。

| E-27 | 外层服务端二进制内**内嵌了 40+ 个 agent 模板**：PE（amd64/i386，console/gui）与 ELF（amd64/arm64），单个 5–12 MB。这正是面板 `stageless/<tp>_<arch>`、`dll/<tp>_<arch>` 要取的文件 | `scripts/extract-templates.py`（PE 头 + program header 推算镜像边界） | M2 载荷链路：可直接拿真模板做 patched 回归 |
| E-28 | 内嵌模板本身也是 Go 二进制，`.gopclntab` 可用同一脚本恢复：实测 amd64 console 模板 **9482 个函数 / 3444 个有名字**（用户代码被 garble，方法名少量保留，如 `(*Config).Clone`） | `scripts/go-symbols.py` + 手工定位 pcHeader | 反向对照 agent 行为时可比外层更容易定位 |

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
| U-09 | 抓真实下发瞬间的链路字节（KCP Main 帧内层），确认是 24B 记录还是别的封装 |### E-29 官方载荷的配置区（动态取证，R13）

方法：把 `/home/kali/go/bin/vshell` **硬链**到独立目录（同一文件系统、零拷贝），
让它读自己的 `conf/setting.conf` 起第二实例（:39083），再调用
`/api/download/listen` 拉真实产物，与二进制里嵌入的模板逐字节对比。

得到的事实：

| 项 | 结论 |
|---|---|
| 载体 | 模板里预留 **20480 字节全 `0x19`** 的占位区，下发时**原地覆盖**（不是追加 trailer） |
| 布局 | `[16B IV][AES-128-CBC 密文]`，共 20480 字节 |
| 密钥 | `key = IV = md5(vkey)` 的**原始 16 字节**（即 AES-128） |
| 明文 | JSON + `0x00` 填充到 20464 字节（**无 PKCS#7**） |
| JSON | `{"server":"host:port","type":"tcp|kcp|ws|dns|oss","vkey":...,"proxy":...,"salt":...,"l":true,"e":false,"d":30,"h":10}` |
| 确定性 | 同参数多次请求字节完全一致；改 `salt` 只改 JSON 的 salt 字段（IV 只依赖 vkey） |

模板名规律（从二进制字面量枚举）：`stageless/<tp>_<goos>_<goarch>[.exe]`、
`dll/<tp>_<goos>_<goarch>.dll`，其中 tp ∈ tcp/kcp/ws/dns/oss（**没有 http**），
另有 `stageless/ebpf_<tp>_linux_<arch>`。Windows 产物带 `.exe`，但服务器拼出的
相对路径不带后缀（因此 `arch=windows_amd64` 会返回 `open ...: file does not exist`）。

字段语义（第二组取证：另建 listener `PingInterval=7 / DisconnectTimeout=99`，
下载后解出 `"d":99,"h":7,"l":false`）：

| 字段 | 含义 |
|---|---|
| `l` | listen 独立载荷为 `true`，stageless 为 `false` |
| `d` | DisconnectTimeout（秒） |
| `h` | PingInterval（秒） |
| `e` | 仍待定（两例均为 `false`） |

验证产物：`testdata/original/payload/config_region_tcp_linux_amd64.bin`
（20480 字节配置区）+ `config_region_meta.json`（请求参数与明文），
`c2engine/agentconfig_test.go` 用它做**黄金向量**：复刻端编码器输出与官方逐字节一致。

### E-30 载荷配置区的补充取证（R14）

在同一套"独立实例 + 产物解码"手段上继续做了四组对照：

| 实验 | 结果 |
|---|---|
| `/download/stageless?id=1`（tcp 监听器） | `{"server":"","type":"tcp","vkey":"vk2","proxy":"","salt":"","l":false,"e":false,"d":99,"h":7}`；与模板对比**只有配置区变化** → 文件里**没有 argv 条目** |
| `/download/listen` 带 `proxy=` / `Proxy=` / `proxyUrl=` / `host:port` | `proxy` 恒为空串 → 官方**不把该参数写进配置区** |
| `/download/stageless` 用 oss 监听器 | `{"server":"https://oss.example.com","type":"oss",...,"d":11,"h":5}` → oss 的 server 取 OssUrl，d/h 取监听器 |
| dns 载荷（ebpf 模板族） | `type":"dns"`，`e` 仍为 `false` |

结论补充：
- `server` 的来源：`listen` 用请求里的 host:port；`stageless` 用监听器的 ListenAddr
  （未配置时为空串）。
- `proxy` 字段存在但当前版本恒为空（参数被忽略），复刻端已对齐为写空串。
- `e` 在本轮所有组合（tcp/kcp/ws/dns/oss × listen/stageless）里都是 `false`，
  语义仍未确定；`l` 只与下载模式相关。
- **官方载荷文件里没有 argv 条目**：复刻端 `PoArgvEntry` 追加的 2 字节 key 记录
  属于复刻端构造，已登记为偏差（不再用于官方格式产物）。

### E-31 互操作实验与传输模式清单（R15）

目标：让**官方 agent** 连上**复刻端服务端**，直接观察它的线格式。

做法与结果：

1. 复刻端起 http 监听器（:18601，vkey `INTEROP-VK`），并打开
   `VSHELL_CAPTURE=<file>` 取证开关（复刻端新增，见 `c2engine/listener.go`）。
2. 用官方实例 `/download/listen?tp=tcp&host=127.0.0.1&port=18601&vkey=INTEROP-VK`
   生成真实 agent 载荷（7.8 MB ELF，配置区已写入）。
3. 运行该 agent：**立即退出**（exit 0，无 stdout/stderr），未向 18601 发起任何连接，
   也没有监听端口 → 本轮无法采集线格式。

同时确认的确定性事实：

- **官方没有 `http` 传输**：模板清单只有 `tcp/kcp/ws/dns/oss`（+`ebpf_*`）。
  请求 `tp=http` 时返回 `open stageless/http_<arch>: file does not exist`，
  所以"面板上的 http 模式"在载荷侧对应的是 `tcp`。
- 载荷里 `l:true`（listen 独立模式）的 agent 字符串含
  `server start, the bridge type is %s, the bridge is %s`，说明它可能**先起监听**
  而非主动连接；`l:false`（stageless）需要监听器真实绑定地址（ListenAddr），
  而官方 `/listener/add` 不接受 Host/Port，未绑定时刻产物里 `server` 为空串。
- `/listener/edit` 读**大写** `Id`，缺省返回 `id is null`（复刻端已对齐）。

结论：官方 agent 在本机环境无法启动（疑似反虚拟机/环境检测），互操作取证改走
"静态字符串 + 产物对比"路线；`VSHELL_CAPTURE` 开关保留，等有可用 agent 时直接用。

### E-32 DelFile 的 gdb 尝试（R17，未成功）

方法：DB 里预插一条 client 记录（`Status=1`）让 `GetClient(1)` 成功，再 attach gdb
到第二实例，断在 `ClientController.DelFile`(0x1822d80) 与其调用的
`0x1820680`（推测是 dispatchCmd 的封装）。

观察：

- `0x1820680` 确实被命中，但返回 `rax=0`（nil）→ 客户端未真实连接，
  代码走了 `JsonErr("connection error")` 分支，**命令并未真正构造/下发**。
- 断点处寄存器里看到的"字符串" `0x1b2970d`（len=5）实际是一片 Go **errno 字面量**
  表（`E2BIG`、`EACCES`…），说明"lea 出来的地址就是命令字符串"这个推断不成立。
- 结论：`DelFile` 的命令很可能在 `0x1820680` **内部**由 id/paths 拼出来，
  静态读调用点拿不到；需要：
  1. 可运行的官方 agent（见 E-31）以获得真实线格式，或
  2. IDA 反编译 `0x1820680` 与其下层函数。

### E-33 官方 agent 线格式实证（R19，重大）

**背景**：R15 之所以跑不起官方 agent，是因为误用了 `listen` 模式载荷——它是**绑定监听**
型（`l:true`），会 `bind()` 配置里的地址，端口被占就直接 `exit_group(0)` 静默退出
（`strace` 里 `bind(...) = -1 EADDRINUSE` → 立刻退出）。改用 **stageless**（`l:false`）
后，agent 会主动 `connect()` 回来。

**方法**：
1. 官方实例 `/listener/add` 支持 `ListenAddr` 与 **`ConnectAddr`** 两个键
   （`Host`/`Port` 被忽略，与早期结论一致）；
2. `stageless` 载荷的 `server` 取自监听器的 **ConnectAddr**；
3. 用一个 Python 假服务器监听该地址，抓 agent 主动送来的帧并解密。

**抓到的线格式**：

```
[u32 LE 长度][12B nonce][AES-256-GCM 密文][16B tag]
key = hex(md5(salt)) 的 32 字节 ASCII（salt 为空 → d41d8cd98f00b204e9800998ecf8427e）
```

**首个记录明文**：`05 00 00 00 34 2e 39 2e 33` = `[u32 LE 5]["4.9.3"]`（agent 版本）。
一条连接上会连发两条相同记录，之后等待服务器回应。

**意义**：复刻端的 `FrameEncrypt`/`FrameDecrypt`（`c2engine/protocol.go`）与官方
**完全一致**（长度前缀、nonce 位置、GCM tag、密钥派生）。已固化为黄金向量测试：
`c2engine/wire_golden_test.go` + `testdata/original/wire/tcp_version_handshake.json`
（4 条真实抓包样本，逐条用复刻端代码解密并比对明文）。

**新增确认的接口约定**：

| 键 | 效果 |
|---|---|
| `ListenAddr` | `/listener/add` 会**真的绑定**该地址（`ss` 可见） |
| `ConnectAddr` | stageless 载荷配置区里的 `server` 取这个值 |

### E-34 握手与心跳记录实证（R20，MITM）

方法：官方监听器绑 `127.0.0.1:18611`，stageless 载荷的 `ConnectAddr` 指向
**Python MITM 代理**（`:18602` → `:18611`），代理逐帧解密并打印。官方 agent 通过代理
**成功上线**（面板 `clientOnlineCount=1`，`HostName=kali`、`OsName=linux_amd64`）。

完整握手（明文；`[n]` 表示 u32 LE 长度前缀）：

| 方向 | 明文 | 说明 |
|---|---|---|
| C→S | `[5]"4.9.3"` ×2 | agent 版本（同一连接连发两条） |
| S→C | `[32]<hex>` | 服务端挑战串（十六进制文本） |
| C→S | `[32]<hex>` | agent 应答串 |
| S→C | `[4]"sucs"` | 认证通过 |
| C→S | `[4]"conf"` | 请求配置下发 |
| S→C | `[1]"\x01"` | 许可（1） |
| C→S | `"conf \x01\x00\x00{client json}"` | **上线记录**：整条 client 记录（Id/IsConnect/VerifyKey/Tp/Addr/LocalIP/UserName/HostName/OsName/ProcessName/…与 `/api/client` 字段一致） |
| S→C | `[1]"\x01"` | 接受 |
| S→C | `[16]` 会话 ID | 如 `IVAoXA2etic7VgbY` |
| C→S | `[4]"main"` | 注册主通道 |

心跳（每 5 秒）：

| 方向 | 明文 |
|---|---|
| C→S | 7 字节 `00 ff ff ff ff 1e 00` |
| C→S | 30 字节 RFC3339 时间戳 |
| S→C | 7 字节 `08 ff ff ff ff 1e 00`（应答） |

要点：
- 记录 = `[u32 LE 长度] + 文本/JSON`，**不是**纯二进制结构；命令与状态都走文本词汇
  （`sucs`/`conf`/`main` 等）。
- `conf` 后的 `\x01\x00\x00` 三个字节含义待定（可能是通道号/标志）。
- 证据：[testdata/original/wire/mitm_session_2026-11-20.log](mitm log 全文) 与
  `handshake_summary.json`。


