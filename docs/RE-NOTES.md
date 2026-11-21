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

### E-35 任务记录与结果格式实证（R21，MITM）

在 E-34 的 MITM 链路上（官方 agent 通过代理上线成功），从面板下发各种操作，
抓到了**服务端→agent 的任务记录**与**agent→服务端的结果记录**。

明文信封（解密后）：

```
[u32 LE 长度][type u8][channel u32 LE][payload]
type: 0x06 服务端 ACK / 0x03 数据 / 0x05 进度 / 0x07 结束 / 0x01 客户端 ACK
```

服务端下发一例（shell `id`）：

```
S→C  06 01 00 00 00                        ACK chan=1
C→S  01 01 00 00 00                        客户端 ACK
S→C  03 01 00 00 00 b7 00                  数据：chan=1，长度 183
S→C  b3 00 00 00 {task json}               183 字节 JSON
```

任务 JSON（字段顺序固定）：

```jsonc
{"ConnType":"v","Host":"v","LocalProxy":false,"RemoteAddr":"v",
 "Req":{"Pass":"v","Type":"M","File":null,"Z1":"id","Z2":"","Z3":"","Z4":"","Z5":""},
 "Option":{"Timeout":5000000000}}   // 纳秒
```

**Req.Type 类型码（全部实测）**：

| 码 | 操作 | 参数 |
|---|---|---|
| `M` | 执行命令 | Z1 = 命令行 |
| `B` | 列目录 | Z1 = 目录 |
| `C` | 读文件 | Z1 = 完整路径 |
| `A` | 磁盘信息 | — |
| `D` | touch/新建 | Z1 = 路径 |
| `J` | mkdir | Z1 = 目录（末尾带 `/`） |
| `I` | 移动/重命名 | Z1 = Z2 = `"src\|<-dst/"` |
| `DSELF` | 删除文件 | — |
| `Upload` | 上传 | `File` = base64 内容，Z1 = 目标路径 |

结果回传（agent→服务端，type=0x03 数据）：payload 形如 `->|<输出>|<-`
（例：`->|uid=1000(kali) …|<-`）。

> 附带解答了 R17 的疑问：gdb 里 `0x1b2970d` 处看到的 `"DSELF"` 确实是删除任务的类型码，
> 当时把它当成 errno 字面量是误判。

**复刻端成果**：`c2engine/official_task.go` 定义同名结构并能**逐字节**重现抓包 JSON；
`official_task_test.go` 用 9 条真实样本做黄金向量（解码→重编码后与原串完全一致）。

证据：[testdata/original/wire/task_records.json](抓包解析)、
`mitm_task_session_2026-11-20.log`（原始记录）。

### E-36 认证算法与通道注册名（R22）

在假服务器上做对照实验（同一个官方 agent 载荷，逐个变量替换）：

| 服务端下发的挑战 | agent 反应 |
|---|---|
| 随机 16 字节 hex | **无响应**，5 秒后重连 |
| 抓包里出现过的常量 `a89a6dc509abfd63bcbed973e249120e` | 回 `224edc5281ea0aaeece8382983dce811` |

两条直接结论：

1. **挑战 = `md5(版本号)`**：`md5("4.9.3") = a89a6dc509abfd63bcbed973e249120e`
   （已用 15 个候选串暴力比对确认，唯一命中）。
2. **应答 = `md5(vkey)`（十六进制）**：
   - E-34 里 `md5("VK-P") = 9e09b0be529f6a4206198ad9dc340124` = 抓到的应答；
   - R22 里 `md5("VK9")  = 224edc5281ea0aaeece8382983dce811` = 现场应答。
   即：服务器用 md5(版本) 证明自己是对端，agent 用 md5(vkey) 证明自己持有密钥。

**通道注册名**（从 E-34 的三条连接可见）：认证通过（`sucs`）后，agent 会按通道发送
`conf`（配置/主通道，随后传整条 client JSON）、`main`（主命令通道）、`chan`（数据通道）。

**仍卡住的一步**：假服务器按抓包顺序回 `sucs` 后，agent **不再发送通道名**，5 秒后重连。
已排除：挑战内容、nonce 用计数器、会话 ID 提前下发。怀疑还有一条服务的记录或
对 nonce/序号的额外校验，下一步用 IDA 看 agent 的认证后续处理（模板 ELF 里
应能找到 `sucs` 字符串的 xref）。

### E-37 假服务器打通认证，并确认通道模型（R23）

在 E-36 的基础上把假服务器写成完整状态机（版本→挑战→应答→`sucs`→通道名→`conf` JSON）：

- **认证确实走通**：日志里出现 `C->S 'conf'`（此前误判成"agent 不回应"，
  实际是脚本提前发了多余记录打断流程）。
- 服务端应答顺序（按抓包复原）：
  1. 读到两条 `[5]"4.9.3"` → 发挑战 `md5("4.9.3")`；
  2. 收到 32 字节 `md5(vkey)` → 发 `"sucs"`；
  3. 收到 `"conf"` → 发 `[1]\x01`；
  4. 收到 `"conf \x01\x00\x00{client json}"` → 发 `[1]\x01` + 16 字节会话 ID；
  5. 其它连接分别注册 `"main"` / `"chan"`。
- **不稳定现象**：agent 会同时开 3 条连接，其中部分连接在收到挑战后不回应、
  5 秒后重连（重连后又能走完认证）。同样的代码有时第一条就通过、有时第三条才通过，
  属于**竞态**而非协议错误。下一步做 Go 版服务端时用"每条连接独立状态机 + 容忍重连"
  的写法，并对照抓包确认是否需要额外记录。

### E-38 官方 agent 成功连上复刻端（R24）

把 E-33…E-37 的协议实现成 Go 服务端（`c2engine/official_tcp.go`，CLI：
`vshell-server official-tcp -listen 127.0.0.1:18650 -vkey VKGO`），再用官方载荷连它。

**跑通的部分**（日志实证）：

```
conn from 127.0.0.1:...
<- "\x05\x00\x00\x004.9.3"   ×2
-> 挑战 md5("4.9.3")
<- "5652f1ea25d4661b1da0ab502d3a194d"   = md5(vkey)
-> "sucs"
<- "conf"
<- "conf \x01\x00\x00{client json}"
official-tcp: client registered: host=kali os=linux_amd64 user=kali
```

即**官方 agent 通过了复刻端的认证并上线**，复刻端解析出它上报的整条 client 记录。

**一个关键时序**：回挑战必须**稍等 30ms**。立刻回挑战时官方 agent 常常不理睬、
5 秒后重连（Python 原型里表现为"前两条连接不回应"）。加上延迟后稳定通过。

**尚未打通**：agent 一直只发 `conf`，从不发 `main`/`chan`，因此任务下发仍报
"no main channel"。抓包里 `main` 出现在**第二条连接**上，说明 conf 之后还有一条
服务端记录（或会话确认）驱动它进入主通道；下一步用 IDA 看 agent 里 `main`
字符串附近的读取逻辑。

### E-39 main 通道仍未触发的排查（R25）

对照实验与结论：

| 实验 | 结果 |
|---|---|
| 回挑战前加 30ms 延迟 | ✅ 稳定通过认证并收到 `conf` |
| 回 `sucs` 前加 20ms 延迟 | ❌ agent 连 `conf` 都不发（说明 sucs 要立刻发） |
| 认证后再补 `\x01` + 会话 ID | ❌ 不触发 main |
| 挑战复用 nonce / 计数器 nonce | ❌ agent 无响应 |

用抓取到的**服务端真实帧**做对照（R25 新增非ces 级抓包）：服务端每帧 nonce 都是随机，
plaintext 序列与我们完全一致（`md5("4.9.3")` → `sucs` → `\x01` → 16 字节会话）。

现象：conf 通道完整走通（client 记录已解析），但 agent 后续连接**认证后不再发
`main`/`chan`**；而在真实服务端上，agent 会在第二条连接认证后立刻发 `main`。

剩余可能：真实服务端在 conf 完成后还发了一条我们没抓到的记录（例如面板侧的
"start"指令），或 agent 依据会话 ID 的某些位决定通道用途。下一步：用 MITM 长时间
挂住真实链路，观察 conf 之后是否还有服务端→agent 记录。

### E-40 带时间戳的抓包与 conf 连接 EOF 现象（R26）

给 MITM 加上时间戳与连接编号后，看到了两条此前被忽略的事实：

1. **真实服务端回 `sucs` 有明显延迟**：
   ```
   [1]C>S md5(vkey)   ...652
   [1]S>C 'sucs'      ...018   ← 366ms 后
   ```
   conn2 上同样有 266ms 延迟。复刻端据此加了 `AuthDelay = 300ms`
   （此前"立即回"或"只等 20ms"都不稳定，20ms 时 agent 连 conf 都不发）。

2. **会话 ID 的形态**：真实值是 `FQ0022pIHROm5ibh` —— 16 个字符、大小写混合，
   符合 **12 字节随机数的 base64**（12 % 3 == 0，无填充）。复刻端改为
   `base64(12 随机字节)`。

仍然存在的差异：**复刻端发完会话 ID 后，agent 立即 EOF 关闭 conf 连接**；
真实服务端上该连接会保持，并在随后（+45ms）另开一条连接发 `main`。
因此 agent 一直重复建 conf 通道，从不发 `main`。

下一轮方向：把真实服务端在会话 ID 之后**是否还有字节**查清（用只读探针在
conn1 上挂住更久），或对 agent 的 conf 处理做定点反编译。

### E-41 官方 agent 模拟器（R27）

为了在没有原版载荷的机器上验证 `official-tcp` 服务端，实现了**官方 agent 模拟器**
（`c2engine/official_agent_sim.go`，CLI：`vshell-server official-agent-sim`）：
按抓包顺序完成 版本×2 → 挑战 → `md5(vkey)` → `sucs` → `conf`/`main`/`chan` 三条连接，
收到任务后执行并回 `->|输出|<-`。

实测（本机）：

```
$ vshell-server official-tcp -listen 127.0.0.1:18660 -vkey VKGO -cmd id
official-tcp: client registered: host=sim-host os=linux_amd64 user=sim-user
official-tcp: main channel registered from 127.0.0.1:38200
official-tcp: result: ->|uid=1000(kali) ...
command "id" output: ->|uid=1000(kali) ...

$ vshell-server official-agent-sim -addr 127.0.0.1:18660 -vkey VKGO
sim done: registered=true tasks=1 results=1
```

**意义**：证明服务端一侧的「认证 → 通道注册 → 任务下发 → 结果回传」全链路正确；
与**真官方 agent** 的差异只剩「它为什么不发 `main`」这一处（见 E-40）。
模拟器同时进入 e2e（§7p）作为回归守卫。

### E-42 连接生命周期对照：main 通道差在"认证后是否被放弃"（R28）

带 OPEN/EOF 事件的 MITM 观察到真实链路（R28 抓包）：

```
622.634 [1] OPEN
623.574 [1]C>S 'conf   {json}'
623.617 [1] EOF            ← conf 连接在收到会话后**同样被 agent 关闭**（此前误以为只有我们会）
623.618 [2] OPEN
624.606 [2]C>S 'main'      ← conn2 打开后约 1s 才声明 main
624.606 [3] OPEN
625.058 [3]C>S 'chan'
```

结论：
- conf 连接关闭是**正常行为**，不是我们的缺陷（R26 的"EOF 差异"结论作废）。
- 真正的差异在**第二条连接**：真实链路上 agent 认证后等待约 1s 再发 `main`；
  复刻端上它认证后**立刻放弃该连接**、再开新连接（日志里 `authenticated` 后紧跟
  新的 `conn from`）。

已排除（R27/R28）：`sucs` 延迟（统一 300ms 与仅首连接延迟都试过）、会话 ID 形态
（hex / alnum / base64 / base64url）、认证后补发 ack 或会话、nonce 复用与计数器。

目前唯一未验证的差异方向：`sucs` 帧之后 agent 是否还在等**某个字节级细节**
（例如 GCM tag 位置或长度前缀的写法），需要抓取真实服务端 `sucs` 帧的**完整十六进制**
逐字节比对（R26 的抓包只记了明文）。

### E-43 回放实验：用真实服务端的帧能让 agent 注册 main/chan（R29）

把真实服务端的帧（挑战/sucs/ack/会话）**原样十六进制回放**给官方 agent：

```
[1] conn → challenge(replay) → C>S 'conf' → C>S 'conf \x01\x00\x00{json}' → EOF
[2] conn → challenge(replay) → C>S 'main'      ← *** MAIN ***
[3] conn → challenge(replay) → C>S 'chan'      ← *** CHAN ***
```

**结论：agent 完全可以从我们这边被驱动到 main/chan** —— 之前的僵局不是"缺少某条记录"，
而是"我们生成的帧与真实帧存在尚未定位的差异"。

同一脚本改用**新生成的**帧（随机 nonce，同样密钥、同样明文）时，agent 连挑战都不回应，
说明差异落在**帧的字节层面**（不只是明文内容）。

候选差异（下一步逐项排除）：
1. GCM nonce 的取值/派生方式（是否与上一帧关联、是否有固定前缀）；
2. 4 字节长度前缀与实际帧长的关系（我们按 12+ct+16 计算）；
3. 密钥是否真的就是 `hex(md5(salt))` 的 32 字节 ASCII（能解密成功说明大概率对，
   但也可能 agent 对**加密方向**用了另一个密钥/IV）。

证据：`/tmp/m8.log` 抓到的服务端帧十六进制（挑战 64B / sucs 36B / ack 33B / 会话 48B）。

### E-44 nonce 分析、会话格式敏感性与 flakiness（R30）

本轮三个结论：

1. **nonce 无规律**：抓包里两侧每帧 nonce 都是 12 字节随机（挑战/应答/sucs/ack/会话
   各不相同，无计数器、无链式派生），因此 R29 的"新帧被拒"更可能是脚本里
   `\x01` 转义写错导致的假象，**不能**据此断定 nonce 派生有问题。

2. **sucs 延迟在每条连接上都存在**：精确时间戳显示
   - conn2：应答 .279 → sucs .604（325ms）
   - conn3：应答 .606 → sucs .919（313ms）
   复刻端已按 300ms 统一延迟对齐。

3. **会话 ID 必须是 base64 文本**：改成"纯字母数字 16 字符"后，agent 连 `conf` 都不再
   上报；改回 `base64(12 字节)` 才恢复。说明 agent 会 base64 解码并校验字节数。

另外确认了一个长期现象：官方 agent 对挑战的**响应本身不稳定**（同一二进制、同一服务端，
有时第一条连接就应答、有时连续多条都不应答），这与 R22 起观察到的 flakiness 一致，
排查时需要考虑重试次数，而不是把单次失败当作协议错误。

### E-45 探针脚本与 conf 前缀字节的 bug（R31）

新增可重复实验脚本 `scripts/official-probe.sh`：硬链原版二进制起独立实例 → 生成官方载荷
→ 连**复刻端** official-tcp（20s）→ 统计 连接/认证/conf/main/chan 次数。`KEEP=1`
保留现场日志。

第一次跑出来 conf=0，但日志里明明有 `<- "conf" ...` → 逐个连接看发现 agent 的
conf 记录是：

```
conf)\x01\x00\x00{"Id":0,...}
```

即 **"conf" 后面的分隔字节会变**（早期抓包是 `0x20` 空格，这次是 `0x29` `)`），
而复刻端当时用 `strings.HasPrefix(plain, "conf ")` 判定，字节一变就整条被忽略 →
不回复 ack/会话 → agent 关闭连接重来。

修复：只要求前 4 字节是 `conf`、且记录长度 > 4，JSON 从第一个 `{` 开始取。
修复后 conf 立即恢复（三次运行分别 3/13/23 次），但 `main` 仍为 0。

下一步：把"分隔字节"的语义搞清楚（可能携带通道号/序号），并按真实服务端对它的
应答方式回复——这很可能就是 agent 不肯进入 main 的原因。

### E-46 conf 分隔字节再观察 + conn2 应答异常（R32）

1. **conf 分隔字节果然会变**：本轮实测到 `0x20`(空格)、`0x22`(`"`)、`0x29`(`)`) 三种，
   印证 R31 的修复（只认前 4 字节 `conf`）是必要的。

2. **真实服务端的 conf 完成序列**（原始字节抓包）：
   ```
   C>S 328B  conf)\x01\x00\x00{json}
   S>C  81B  = ack(33B) + session(48B)      ← 两条帧在一个 TCP 段里
   C>S EOF                                  ← agent 主动关闭 conf 连接
   C>S conn2: 版本×2 → 挑战 → 应答(md5(vkey)) → sucs → 'main'
   ```
   与复刻端的行为逐项一致。

3. **一个待解异常**：某次运行里 conn2 的应答不是 `md5(vkey)`（`9e2c5a4d…`），
   而用固定会话（`AAAABBBBCCCCDDDD` / `ZZZZYYYYXXXXWWWW`）复测时，conn1/conn2
   的应答又都等于 `md5(vkey)`。说明存在某个条件会让 agent 用**另一种派生**应答，
   需要在受控条件下（固定会话 + 多次采样）继续定位。

新增调试开关：`VSHELL_FIXED_SESSION`（把会话 ID 固定成已知值，便于对比公式）。

### E-47 双向 flaky 与四个失败实验（R33）

**1. flaky 是双向的**：对着**真实服务端**抓包时，也多次出现"版本→挑战→应答→sucs→
agent 什么都不发、10 秒后重连"。所以"不发 conf"不是复刻端独有现象，而是 agent 侧
本身不稳定；排查必须看**多次尝试的统计**，不能凭单次结果判定。

**2. 复刻端 100 秒浸泡数据**（`scripts/official-probe.sh 5`）：

```
连接=6  认证=2  conf=12  main=0
连接=12 认证=5  conf=24  main=0
连接=17 认证=8  conf=35  main=0
连接=22 认证=10 conf=36  main=0
连接=29 认证=13 conf=39  main=0
```

即 conf 能**反复完成**，但 agent 每次都从头重试，不进入 main。

**3. 本轮试过、均无效的四个对齐**：

| 实验 | 结果 |
|---|---|
| ack 与会话合并成**单次 write**（真实服务端是一个 81 字节段） | conf 照旧重复，main 仍 0 |
| 在 ack 里**回显 conf 的分隔字节**（0x20/0x22/0x29 可能携带通道号） | 无变化 |
| 会话 = `base64(md5(vkey)[:12])`（验证是否与 vkey 绑定） | 无变化 |
| 固定会话（`VSHELL_FIXED_SESSION`）| 应答稳定为 md5(vkey)，但 main 仍不出现 |

**4. 结论**：conf 的"完成"在协议字节层面已与真实服务端一致，但 agent 仍判定未完成。
剩余可疑点只剩 **会话值的语义**（内容/长度/编码之外的东西），或者 agent 侧存在
一条我们尚未观察到的判据。下一步建议直接用 IDA 反编译 agent 里处理 conf JSON
响应（`\x01` + 会话）的函数，而不是继续黑盒试探。

### E-48 字符串→函数映射（R35，agent 模板 ELF）

新增工具 `scripts/find-string-ref.py`：在 garble 过的 Go 二进制里，用
「字符串偏移 → `LEA reg,[rip+disp]` 引用点 → pclntab 函数」这条链路定位逻辑，
不需要 IDA 也能把协议字符串映射到函数。

在 agent 模板（12MB ELF）上的结果：

| 字符串 | 结论 |
|---|---|
| `conf` | 引用集中在函数 `0x643600`（两处：0x644009、0x64412a） |
| `main` | 引用在函数 `0x644860`（0x644945） |
| `chan` | 引用在函数 `0x644aa0`（0x644f39） |
| **`sucs`** | **整个二进制里不存在这个字面量** |
| `conf `（带空格） | 也不存在（与 R31/R32 观察到的"分隔字节会变"一致） |

两个直接推论：

1. **agent 不校验认证应答的文本**（没有 "sucs" 字面量），所以复刻端回什么内容
   都不会影响认证判定——卡点不在 `sucs`。
2. `conf`/`main`/`chan` 三个处理函数是连号（0x643600 / 0x644860 / 0x644aa0），
   构成一条通道注册状态机；下一步用 IDA 对这三个函数做 Hex-Rays 反编译，
   重点看 `0x643600` 里对**会话记录**的处理与后续跳转条件。

### E-49 纠正 E-48：字符串→函数映射在 agent 上不成立（R36）

用 IDA 打开 agent 模板验证 E-48 的结果时发现：`0x643600` 其实是
`vendor/golang.org/x/crypto/cryptobyte.(*Builder).addLengthPrefixed`，
与协议无关 —— **E-48 给出的 conf/main/chan 函数地址是假阳性**。

原因：`find-string-ref.py` 原来在**任意字节位置**匹配 `48 8d 05 ...` 并计算 RIP 相对目标，
命中的往往只是某条更长指令内部的字节。

修正：脚本现在先用 `objdump -d` 取**全部指令起始地址**（本例 159.5 万条），
只接受"引用点正好是指令起始"的命中。重跑结果：

| 字符串 | 有对齐过滤后的结果 |
|---|---|
| `conf` | **没有任何直接引用**（此前的两处是假阳性） |
| `sucs` | 字符串本身不存在（该结论仍然成立） |
| `screenshot` / `download` | 字符串存在，但**没有直接 LEA 引用** |
| `filelist` | 不存在（原版 agent 的 JSON 命令词汇与复刻端不同） |

结论：**agent 模板里的字符串不是通过直接 LEA 访问的**（可能经由基址+偏移表或
garbled 字面量访问方式），所以"字符串→xref→函数"这条路径在 agent 上不可用。
后续要定位协议逻辑只能靠 IDA 的人工/半自动分析（按调用图与常量特征），
或者换一台能跑官方 agent 的机器做动态跟踪。

### E-50 字符串存储方式的实测结论与"已排除假设"清单（R37）

**1. 服务器二进制的 UI 字面量并非连续存放**

用已知常量验证：登录失败文案在面板上是 `Incorrect account or password！`，但二进制里
搜不到 `Incorrect account`、`or password`、`Incorrect account or password` 任何一段
（`password` 本身只出现 3 次，且不在同一区域）。说明这类字面量由 garble 做了
**字符串混淆**（运行时拼装/解码），不能靠 `strings` + 交叉引用定位。

对照：`stageless/tcp_linux_arm`、`dll/ws_windows_i386.dll` 这类**嵌入模板路径**是明文，
因为它们来自 `embed.FS` 的文件名表，而不是普通字符串常量。

**2. 因此 `string-xrefs.py` / `find-string-ref.py` 的正确用法**

- 只能用于**明文常量**（嵌入资源路径、panic 文案等）；
- 直接扫 `48 8d 05` 会命中"长指令内部字节"，必须用 `objdump` 的指令起始地址过滤；
  两个脚本本轮都已加该过滤（`string-xrefs.py` 新增 `_START_CACHE` + objdump 对齐）。
- 过滤后大量旧命中消失 → 说明此前若干"函数定位"结论不可靠（E-48 已作废）。

**3. 已排除的假设（累计，避免重复尝试）**

| # | 假设 | 结论 |
|---|---|---|
| 1 | 挑战可以是随机值 | ❌ 必须等于 `md5(版本号)` |
| 2 | 应答是固定 `md5(vkey)` | ⚠️ 首连接成立；后续连接偶见不同值（未解） |
| 3 | `sucs` 内容需要精确匹配 | ❌ agent 里没有 `sucs` 字面量 |
| 4 | `sucs` 前需要延迟 | ⚠️ 首连接需要 ~300ms；其它连接无定论 |
| 5 | 会话 ID 可以是任意 16 字符 | ❌ 必须可 base64 解码为 12 字节 |
| 6 | 会话需与 vkey 绑定 | ❌ 用 `md5(vkey)[:12]` 无改善 |
| 7 | ack 与会话分开写有问题 | ❌ 合并成单次 write 无改善 |
| 8 | 需回显 conf 的分隔字节 | ❌ 无改善 |
| 9 | nonce 有派生规律（计数器/链式） | ❌ 全随机 |
| 10 | agent 缺少某条服务端记录 | ❌ 原始字节对照显示我们的序列与真实服务端逐项一致 |
| 11 | 用字符串 xref 定位 agent 协议逻辑 | ❌ agent 里相关字符串无直接 LEA 引用 |

**4. 结论**：M1 的最后一环（官方 agent 注册 main）在黑盒与静态两条路上都已到达边界。
剩下可行的只有：IDA 人工分析（按调用图定位 net/aes 附近的状态机）或换一台干净机器
做动态调试（把 gdb 断点下在 `net.Conn.Write` 的调用者上，看它在什么条件下写 "main"）。

### E-51 保存设置会踢掉所有登录（R38，e2e 覆盖发现）

给 e2e 补端点覆盖时，`setting/edit` 之后 `setting/get` 一直拿不到新值。排查发现：

1. `UpdateConfig` 写完文件会调用 `SyncSettingsFromConfig()`；
2. 该函数在 `web_jwt_secret` 为空时**每次重新生成随机密钥**；
3. 于是"保存一次设置"就把 JWT 密钥轮换了 —— **此前签发的所有 token 全部失效**
   （表现为接口返回 `{"code":-1,"result":null}`）。

修复三处：

| 位置 | 改动 |
|---|---|
| `utils/config.go` | 密钥为空时优先沿用已有单例的密钥，其次生成一次并写回 cfg |
| `main.go` | 启动时（`loadConfig`）就调用 `SyncSettingsFromConfig()`，保证从第一个请求起密钥稳定 |
| `controllers/setting.go` | `Edit` 成功后同步更新内存单例，`Get` 立即能读到新值 |

回归测试：`TestUpdateConfigKeepsJWTSecretStable`（保存设置后旧 token 仍有效）、
`TestSettingEditVisibleImmediately`（保存后立刻可读）。

### E-52 客户端列表的筛选/搜索两个真 bug（R39）

给 `client/list` 补筛选断言时抓到两个问题：

1. **`status=1`（仅在线）返回空**：引擎侧用 `NowConn > 0` 判定在线，但 HTTP/WS 传输
   只有 `Status` 会被 checkin 置位、`NowConn` 恒为 0 → "在线"筛选永远空。
   修复：统一为 `clientOnline(c) = c.Status || c.NowConn > 0`（与面板 `IsConnect` 同源）。
2. **搜索按主机名搜不到**：`clientMatches` 只匹配 `Addr/Remark/OSType/Arch/Version`，
   不含 `HostName/UserName/LocalIP/ProcessName/Location`，且区分大小写。
   修复：覆盖全部展示字段 + 大小写不敏感。

另外把 `sortClients` 的字段名改成大小写不敏感（面板会传 `Id`/`HostName` 这类首字母大写）。

回归测试：`TestGetClientListStatusFilterUsesStatus`、`TestClientSearchAndSortFields`；
e2e §7q 增加 status/search/排序三项断言。

### E-53 离线/缺参语义对齐（R40）

用"负面用例"（不存在的 id、缺参数、非法令牌）对照实时原版，发现两处行为差异：

| 场景 | 原版 | 复刻端（修复前） |
|---|---|---|
| `file/ls`、`file/cat`、`file/getdisk` 对离线客户端 | `connection error` | 静默返回空列表 / 空结果（code 0） |
| `listener/stop` 对不存在的 id | `not found` | `stop success` |

修复：
- `controllers/file.go` 新增 `requireClient(id)`：客户端缺失/离线统一回
  `connection error`，用在 `ls`/`cat`/`getdisk` 等"不经过 dispatchCmd"的入口。
- `controllers/listener.go` 的 `Stop` 先查存在性，缺失回 `not found`（与 `Start` 一致）。

同时确认了原版的几个语义（已写进 e2e §7r 断言）：

| 接口 | 缺失实体时 |
|---|---|
| `listener/start` / `stop` | `not found` |
| `listener/del` | `delete success`（幂等） |
| `tunnel/start` / `stop` | `start error` / `stop error` |

### E-54 竞态检测（R41）

用 `go test -race` 跑全量测试，发现三处数据竞争，其中两处是**生产代码**问题：

1. **`Engine.GetTask` 返回内部指针**
   调用方在锁外读 `task.Status/Result`，而 `UpdateTask` 在锁内并发写 → 竞争。
   修复：在 RLock 内做浅拷贝返回（注意拷贝本身也必须在锁内，第一次改法把拷贝放到
   解锁后，-race 仍然报竞争）。

2. **`TunnelServer.OnOpen` 无保护**
   `engineStartTunnelWithDataPlane` 在 `Start()` 之后才赋值，而为公网连接服务的
   goroutine 会并发读这个字段 → 生产环境的数据竞争。
   修复：改为 `SetOnOpen()` / `getOnOpen()` + 互斥锁；`restartTunnel` 也改用 getter/setter。

   > 改动过程中引入过一个**重入死锁**（在已持有 `ts.mu` 的临界区里再次 Lock），
   > 被 `TestTunnelServerRelaysBytes` 立刻抓出来（OnOpen 不再被调用），已修正为
   > "在既有临界区内直接读字段"。

3. **WS 测试的拆除顺序**（仅测试代码）
   `httptest.Server.Close()` 不等待被 hijack 的 WebSocket 连接，处理器的 defer 里还会
   向 agent 投递 `terminal_close`，于是与下一个测试的 `Engine.Init()` 竞争。
   修复：新增 `closeWS()` 测试助手——先关连接，轮询等待 viewer 注销，再关服务器。

结果：`go test -race ./c2engine ./router` 全绿；`go test ./...` 全绿。


