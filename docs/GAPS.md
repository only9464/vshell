# 已知差距清单（GAPS）

> 目的：把"还没完全对齐原版"的地方集中列出来，每条都带**证据位置**与**下一步**，
> 避免每次讨论都从"还差什么"重新开始。
>
> 最后更新：R17（2026-11-20）

---

## A. 阻塞型（缺环境或缺少可观察对象）

### A-1 任务记录层（opcode + argv / 结果帧）

- **进展（R58）**：**发现两段式认证**——conf 之后的连接用 `md5(会话)` 应答挑战
  （抓包吻合），复刻端已实现并有回归测试；但官方 agent 在复刻端上 conf 完成率很低，
  仍需单独提升这个指标。见 RE-NOTES E-59。
- **进展（R57）**：抓包复核发现我们漏了 sucs 之后那条独立 `\x01`；补上后握手序列与
  抓包逐条一致（agent 先发裸 conf），但 `main` 仍为 0，且后续连接开始不应答挑战。
  详见 RE-NOTES E-58；preack 作为对照开关保留（默认关闭）。
- **进展（R53）**：探针加 `WINDOW` 参数后做了 45s 窗口的统计对照：三种会话取值下
  认证均成功但 `main` 始终为 0，且 agent 会反复重发 conf（11–24 次）——
  **排除"会话内容不被认可"的假设**，卡点在 conf 应答的字段校验上。见 RE-NOTES E-57。
- **进展（R37）**：实测服务器 UI 字面量被 garble 混淆（登录文案在二进制里搜不到），
  只有 `embed.FS` 的模板路径是明文；两个字符串定位脚本都补上了 objdump 指令对齐过滤。
  另把 11 条**已排除假设**整理成表（RE-NOTES E-50），避免重复踩坑。
- **进展（R36）**：用 IDA 复核发现 R35 的"字符串→函数"定位是**假阳性**
  （0x643600 实际是 cryptobyte 的 addLengthPrefixed）；工具已改用 objdump 指令对齐过滤。
  agent 模板的字符串**没有直接 LEA 引用**，该路径在 agent 上不成立 —— 需 IDA 人工分析
  或换机器做动态跟踪。见 RE-NOTES E-49。
- **进展（R35）**：新增 `scripts/find-string-ref.py`（字符串→xref→函数）；
  在 agent 模板里定位到 conf/main/chan 三个连号处理函数（0x643600/0x644860/0x644aa0），
  并发现 **agent 里根本没有 "sucs" 字面量** → 认证应答内容不参与判定。
  下一步用 IDA 反编译这三个函数。
- **进展（R33）**：确认 flaky 双向存在（真实服务端上也会出现 agent 不发 conf）；
  复刻端 100s 浸泡：29 连接 / 13 认证 / 39 次 conf / **0 次 main**。四个对齐实验
  （ack+session 单次写、回显分隔字节、会话绑定 vkey、固定会话）均无效。
  下一步转 IDA 反编译 agent 的 conf 后续处理。
- **进展（R32）**：确认 conf 分隔字节有三种（0x20/0x22/0x29）；原始字节抓包显示真实
  服务端的 conf 完成序列与复刻端逐项一致（ack+session 合并在一个 TCP 段、随后 agent 关连接）；
  发现 conn2 应答偶尔不是 md5(vkey) 的异常，新增 `VSHELL_FIXED_SESSION` 便于受控复现。
- **进展（R31）**：新增可复现探针 `scripts/official-probe.sh`；修掉一个真实 bug——
  agent 的 `conf` 记录里 "conf" 后面的**分隔字节会变**（0x20/0x29），此前用
  `HasPrefix("conf ")` 判定导致整条被忽略、conf 无法完成。修复后 conf 稳定，
  `main` 仍未出现；下一步查该分离字节的语义与应答方式。
- **进展（R30）**：nonce 无规律（R29 的"新帧被拒"疑为脚本转义 bug）；sucs 延迟在每条
  连接上都有（≈320ms）；会话 ID 必须可 base64 解码（12 字节），否则连 conf 都不上报。
  另确认 agent 对挑战的响应本身不稳定（flaky），排查需考虑重试。
- **进展（R29）**：**回放实验成功**——把真实服务端的帧原样回放给官方 agent，它能注册
  `main`/`chan`；换成新生成的帧（同密钥同明文、只有 nonce/密文不同）则连挑战都不回。
  说明差异在**帧字节层面**，而非缺少协议记录。见 RE-NOTES E-43。
- **进展（R28）**：带 OPEN/EOF 的抓包确认 conf 连接被 agent 关闭是**正常行为**；
  真正差异在第二条连接——真实链路认证后等 ~1s 发 `main`，复刻端上 agent 认证后
  立刻放弃该连接。下一步需要真实 `sucs` 帧的完整十六进制做逐字节比对。
- **进展（R27）**：新增**官方 agent 模拟器**（`vshell-server official-agent-sim`），
  证明服务端的认证→通道注册→任务→结果全链路正确（e2e §7p 已覆盖）。
  与真官方 agent 的差异只剩"它为什么不发 main"一处。
- **进展（R26）**：带时间戳抓包发现真实服务端回 `sucs` 有 ~300ms 延迟（复刻端已对齐），
  会话 ID 是 12 字节随机数的 base64；但复刻端发完会话 ID 后 agent 立即 EOF 关闭 conf
  连接（真实服务端不会），因此仍拿不到 `main`。见 RE-NOTES E-40。
- **进展（R25）**：排除多种假设（sucs 延迟、认证后补 ack、nonce 复用/计数器），
  用非ces 级抓包确认服务端每帧 nonce 随机、明文序列与我们一致；`conf` 通道完整走通，
  但 agent 后续连接认证后**不发 main**（真实服务端上会发）。见 RE-NOTES E-39。
- **进展（R24）**：复刻端实现了官方协议的 tcp 服务端（`c2engine/official_tcp.go` +
  `vshell-server official-tcp`），**官方 agent 已能通过认证并上报 client 记录**
  （host/os/user 全部解析成功）。关键细节：回挑战要延迟 ~30ms，否则 agent 不理睬。
  剩余：agent 只注册 `conf` 通道、不发 `main`，任务下发仍报 "no main channel"。
- **进展（R23）**：假服务器按"版本→挑战→md5(vkey)→sucs→通道名→conf JSON"的完整
  状态机**跑通了认证**（日志出现 `C->S 'conf'`）；agent 会同时开 3 条连接，
  部分连接存在"收到挑战不回应、5s 后重连"的竞态，重连后可通过。
- **进展（R22）**：**认证算法已逆出**——挑战 = `md5("4.9.3")`（= `a89a6dc5…`），
  应答 = `md5(vkey)` 十六进制；通道注册名 `conf` / `main` / `chan`。
  假服务器能通过认证但 agent 认证后不发通道名（5s 后重连），还差一条未知校验。
- **进展（R21）**：**任务记录与结果格式已实证并复刻结构**——信封
  `[u32 len][type][channel][payload]`、任务 JSON（`ConnType/Host/Req{Type,File,Z1..Z5}/Option`）、
  9 个类型码（M/B/C/A/D/J/I/DSELF/Upload）、结果 `->|<输出>|<-`。
  `c2engine/official_task.go` + `official_task_test.go`（9 条抓包样本做黄金向量）。
  剩余：把复刻端的**任务下发路径**改成官方格式（当前自家 agent 用自己的 JSON 词汇，
  属有意偏差，改则需同步改 agent）。
- **进展（R20）**：**握手与心跳记录已完全实证**（MITM 代理让官方 agent 成功上线到官方服务端）：
  记录是 `[u32 LE 长度] + 文本`，握手为 `"4.9.3"`→挑战→应答→`"sucs"`→`"conf"`→
  `conf \x01\x00\x00{client json}`→会话 ID→`"main"`；心跳 `00 ff ff ff ff 1e 00`
  + 时间戳。仍缺的是**任务下发/结果回传**两类记录（面板下发命令时官方服务端回报
  "connection error"，需先弄清客户端 id 与通道注册的关系）。
- **进展（R19）**：**帧层已实证一致**——官方 agent 主动连接的握手帧被复刻端
  `FrameDecrypt` 成功解开（`key = hex(md5(salt))`、`[u32 LE 长度][12B nonce][GCM 密文][16B tag]`），
  首个记录明文为 `[u32 LE 5]["4.9.3"]`。已有黄金向量测试
  `c2engine/wire_golden_test.go` + `testdata/original/wire/`。
  仍未对齐的是**记录语义层**（记录类型与字段），需要让假服务器正确回应握手，
  继续录制后续记录。
- **现状**：复刻端自定义了任务命令词汇（文本命令 + JSON 命令），四条传输都能跑通，
  但与原版 agent 的"记录层"未对齐。
- **证据**：`c2engine/wire.go` 的 24 字节记录布局是**复刻端设计**；原版的
  43 槽跳转表（`FUN_010952e0`，opcode 0x01–0x2b）只见于服务端二进制。
- **卡点**：
  1. 官方 agent 载荷在本机运行**立即退出**（疑似环境检测），无法采集真实线格式
     → `docs/RE-NOTES.md` E-31；
  2. 服务端二进制里 DelFile 的命令字符串来自一个**编码字符串表**
     （`0x1b2970d` 处是 Go errno 等无关字面量，说明地址推断有误），
     gdb 断点 `0x1820680` 返回 nil（客户端未连接，走了错误分支）。
- **下一步**：
  - 在干净物理机/虚拟机上跑官方 agent + 复刻服务端（`VSHELL_CAPTURE` 开关已就绪）；
  - 或用 IDA 打开 12MB 的 agent 模板 ELF，直接看记录解析函数。

### A-2 Windows / macOS 平台功能（M7）——**按操作者要求不在本环境适配**

> 操作者明确说明：「目前是 linux 环境，可以不对 windows、macos 特有的功能或项做适配工作；
> 没有的环境可以暂时放弃，并写进文档里」。

- **现状**：Linux 侧功能完整（终端 PTY、文件、屏幕、插件、隧道、MCP 全部在 Linux 验证）；
  Windows/macOS 只有代码分支（管道回退终端、PE 模板提取），未做 UAC/注入/反沙箱。
- **处理**：不在本项目当前范围内继续适配；代码保留 `//go:build windows` 分支，
  避免编译失败即可。若将来需要，按 GAPS 里的清单逐项补。

---

## B. 未实现但可做

| 项 | 现状 | 落点 |
|---|---|---|
| 动态库插件（`.so`/`.dll`/`.dylib`） | agent 明确返回未实现 | 需 cgo + dlopen，或引入 purego；会破坏 `CGO_ENABLED=0` 交叉编译，需评估 |
| `.net` 托管插件 | 明确未实现 | 需要 CLR 宿主，暂不做 |
| Windows ConPTY | 只有管道回退 | 需 Windows 环境 |
| ~~OSS 上传~~ | 文件已能取回服务器并暂存，上传明确报未实现 | **用户明确不实现**（无第三方凭证），见 §F |
| 屏幕流式抓屏 | 每帧起一次子进程（`grim`/`import`/`spectacle`） | 可改为 ffmpeg/x11grab 长连接 |

---

## C. 已知偏差（有意为之，已登记）

| 偏差 | 原因 | 登记位置 |
|---|---|---|
| 载荷尾部追加 `VSCF` 配置块 | 自家 agent 无需 ldflags 即可上线；官方是**原地**写 20480B 配置区 | `docs/PROTOCOL.md` §2.5d、`docs/API.md` §12 |
| 隧道走反向端口转发（`VST1`） | 原版走 C2 长连接的通道帧，复刻端四条传输都是请求/响应式 | `docs/PROTOCOL.md` §3.5 |
| 隧道持久化在 `db/tunnels.json` | SQLite 的 tunnels 表被任务记录占用（沿用原版） | `docs/ARCHITECTURE.md` |
| `proxy` 字段恒为空 | R14 取证：官方三种写法都不写进配置区 | `docs/RE-NOTES.md` E-30 |
| `getMenuList`/`getPermCode` 可用 | 原版 404，复刻端实现（前端需要） | `docs/API.md` §12 |
| `client/add` 可用 | 原版 500 | `docs/API.md` §12 |

---

## D. 未确定的小字段

| 字段 | 观察 | 下一步 |
|---|---|---|
| 载荷配置区 `e` | tcp/kcp/ws/dns/oss × listen/stageless 全为 `false` | 试 upx=true、ebpf 模板、vip 监听器 |
| `PoArgvSeed`（argv 2 字节 key） | 官方产物**无 argv 条目**（R14） | 已不再是阻塞项；保留代码作参考 |

---

## F. 明确不实现（用户无第三方凭证）

> 依据：操作者明确说明"通知以及 OSS 这种需要第三方凭证的功能可以暂时不实现，这些我都没有"。

| 功能 | 现状 | 处理方式 |
|---|---|---|
| OSS 上传（`/file/downloadtooss`） | 文件取回服务器暂存后返回 `oss upload not implemented` | 保持"取回 + 明确报未实现"，不引入 SDK/凭据 |
| OSS 模式监听器（`mode=oss`） | 监听器可建，但没有 OSS 客户端 | 载荷配置区仍按官方写 `type=oss`、`server=OssUrl`（取证已对齐） |
| 企业微信 / 钉钉通知 | `/setting/get|edit` 三个键可读写并持久化到 `conf/setting.conf` | 开关与字段保留（供将来配置），不做实际推送 |

## E. 测试与工程债

| 项 | 说明 |
|---|---|
| UI 自动化 | 只有无头 Chromium 的渲染检查，没有点击级 E2E（需 Playwright） |
| ~~`go test ./...` 偶发 FAIL~~ | **已定位并修复（R23）**：`TestProtocolHTTPCheckinFlow` 用固定 listener ID 99，与全局引擎单例/DB 里的监听器冲突，导致 handler 反查到的 salt 与加密用 salt 不一致 → 偶发 400。改为唯一 ID 并在每次调用前重新登记。连续 30 次单测 + 5 次全量套件均通过 |
| 提取模板体积 | `template-extract/` 约 265MB，未入库（`.gitignore`） |

## G: 本机 4.9.3 实例的可用性边界

`http://127.0.0.1:39082`（admin/123456，需 HTTP Basic）**只能用于 API 探测**：
其 SPA 在本环境渲染不出界面（DOM 里只有 `<svg id="__svg__icons__dom__">` 与一个
`%` 文本，`#/login` 路由已生效但页面内容为空，多次等待 8s 无变化）。

因此前端还原的**唯一可信来源**是仓库里的 `static/assets/*.js`（原版打包产物）：
页面文案、列定义、表单 schema、图标名、组件结构都从这份 bundle 里读出来。
不要用 39082 的截图/观感做对齐依据。

## H: TCP 传输下的文件列表 —— 已修复（R116 发现 / R117 确认）

**结论：上一轮的两个修复之后就通了**，本轮复核（交付包，客户端 #2 走 tcp）：

| 路径 | 结果 |
|---|---|
| `/` | `total=26` ✅ |
| `/tmp` | `total=1198` ✅ |
| `/etc` | `total=314` ✅ |
| `/root` | `total=0`（无权限，符合预期） |
| `terminal/shell` → `echo tcp-file-ok` | `{"output":"tcp-file-ok"}` ✅ |

根因就是那两个已修的 bug：

1. **任务重复下发**（`GetPendingTasks` → `GetAndMarkPendingTasks`）：同一批任务每个
   轮询周期重发，结果与等待中的任务 id 对不上，面板侧因此空结果/超时。
2. **断线不重连**（连接池复用死 socket）：服务端重启后 agent 表面在线、实际不可用。

回归：`scripts/e2e.sh` §7v 已增加「TCP 客户端列目录非空（total > 0）」断言。

## I: 同一 vkey 跑多个 agent → "面板说成功但目标机没变化"（R120 定位并加固）

现象：`POST /api/file/edit`（写文件）返回 `save success`，但目标机上文件没出现。

排查：机器上同时跑着 **3 个** `agent -server tcp://…:8084`（我自己重启时留下的重复进程），
它们都用同一个 vkey checkin 成同一个 client #2。于是：

- 每次 poll 由「谁先连上」决定，任务被不同进程轮流取走；
- `dispatchCmd` 是**投递即返回**（不等结果），所以面板无论如何都回 `save success`；
- 结果就是"看起来成功、实际没人执行完"。

修法与验证：

1. **服务端加单执行者约束**（`tcp_listener.go`）：`owner map[clientID]net.Conn`，
   新的 checkin 接管时关闭旧连接，保证同一 client 只有一个活跃执行者。
2. 只保留一个 agent 进程后复测：写入 `after-owner-fix` 立刻落盘 ✅。

运维提醒：`agent` 不要跑重复实例；重复实例不会报错，只会让任务"随机丢"。

## J: TCP 帧上限导致「跑插件」超时（R124 定位并修复）

现象：在 TCP 客户端上执行插件（`POST /api/runner/runplugin`）等 75s 后返回
`task 248 timed out`；同一个插件在 http 客户端上正常。

原因：插件走的是**任务命令内联 base64**（`{"type":"plugin","data":"<b64>"}`）。
`plugins/sysinfo.elf` 1.6MB，base64 后约 2.2MB，而 agent 的 `readFrame` 上限是
**1MB**——帧直接被判为 `bad frame length`，任务永远拿不到结果。

修复：两侧帧上限对齐到 32MB
（`agent/transport_real.go: maxTCPFrameBytes`、`c2engine/tcp_listener.go: readTCPFrame`）。

实测（TCP 客户端 #2）：

```
{"code":0,"result":{"output":"host=kali os=linux/amd64\nuser=kali uid=1000\npid=…"}}
```

`scripts/e2e.sh` §7v 已增加「TCP 客户端跑插件」断言。

## K: TCP 客户端隧道 —— 已修复（R126 定位 / R127 确认）

**根因**：TCP 客户端只在 checkin 时被标记在线，之后轮询**不刷新在线位**。
巡检 60 秒就把这个正在轮询的客户端判成离线，于是：

- 面板上客户端忽上忽下；
- `dispatchCmd` 直接回 `connection error`，`tunnel_open` 根本发不出去
  （服务端日志：`dispatch tunnel_open for chan 1 failed: connection error`）。

**修复**：`tcp_listener` 每收到一帧就刷新 `Client.SetOnline(true)`（等价于其它传输
的活动刷新）。

**复测**：

| 项 | 结果 |
|---|---|
| 等 70 秒（越过巡检阈值） | 客户端仍在在线列表 ✅ |
| 隧道指向常驻 echo 目标，连续 3 次 | 每次都拿到 `ECHO-28427` ✅ |

e2e §7v 的隧道与插件断言恢复为**硬失败**（不再是 KNOWN-ISSUE）。
