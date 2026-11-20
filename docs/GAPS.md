# 已知差距清单（GAPS）

> 目的：把"还没完全对齐原版"的地方集中列出来，每条都带**证据位置**与**下一步**，
> 避免每次讨论都从"还差什么"重新开始。
>
> 最后更新：R17（2026-11-20）

---

## A. 阻塞型（缺环境或缺少可观察对象）

### A-1 任务记录层（opcode + argv / 结果帧）

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

### A-2 Windows 实战层（M7）

- **现状**：Windows 相关只有管道回退终端、PE 模板提取，无 UAC/注入/反沙箱。
- **卡点**：本机没有 Windows 环境，无法验证。
- **下一步**：交叉编译 + 文档化；功能验证需 Windows 机器。

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
| `go test ./...` 偶发 FAIL | 资源紧张时出现，重跑即过；未定位 |
| 提取模板体积 | `template-extract/` 约 265MB，未入库（`.gitignore`） |
