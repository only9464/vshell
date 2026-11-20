# 原版参考环境（REFERENCE）

> 本仓库的对齐工作有一个**活的原版**可对照：本机运行的 VShell 4.9.3 实例。
> 本文记录如何安全地用它取证、如何把结论落成可回归的契约测试，以及逆向工具链。

---

## 1. 本机原版实例

| 项 | 值 |
|---|---|
| 二进制 | `/home/kali/go/bin/vshell`（ELF x86-64，502 MB，stripped，静态链接） |
| 工作目录 | `/home/kali/go/bin`（`conf/`、`db/`、`plugins/` 都在这里） |
| Web 面板 | `http://127.0.0.1:39082/`（`web_basic_auth=true`，`admin` / `123456`） |
| 数据库 | `db/data.db`（SQLite，WAL） |
| 插件 | `plugins/`：AddUser.dll、fscan.x64.elf、gost.x64.{exe,so}、mimikatz.x64.exe |
| 载荷模板 | 期望在 `<cwd>/stageless/`、`<cwd>/dll/`，当前**不存在**（下载会报 `file does not exist`） |

> 该实例属于操作者本人，用于黑盒对照。**取证时只做只读探测**：除 `login` 外不发写操作；
> 需要验证写接口时用临时对象并立刻删掉（见 §4 的清理要求）。

---

## 2. 黑盒取证流程

```bash
# 1) 抓一份完整的接口基线（默认写入 testdata/original/*.json，token 已脱敏）
FS_BASE=http://127.0.0.1:39082 FS_USER=admin FS_PASS=123456 scripts/probe-original.sh

# 2) 跑契约测试：本实现 vs 原版抓包
go test ./controllers -run TestContract -v
```

抓包覆盖的端点与用途：

| fixture | 端点 | 用途 |
|---|---|---|
| `login` | POST /api/login | 信封 + result 六字段 |
| `userinfo` | GET /api/getUserInfo | 用户信息形状 |
| `dashboard` | GET /api/dashboard/info | **39 个键**（含 icmp/icmpmsg/ip/tcp/udp、load 为 JSON 字符串） |
| `client_list` / `listener_list` / `tunnel_list` | POST …/list | 列表项键集合；**空列表 items 为 null** |
| `runner_list` | GET /api/runner/list | `[{id,name}]`，直接是数组 |
| `setting_get` | GET /api/setting/get | 三个键（钉钉 token/keyword、企微 key） |
| `*_missing` | 客户端相关接口 | 错误文案：`connection error` / `client is close` |
| `download_*` | GET /api/download/* | 模板路径规则与错误文案 |

---

## 3. 已确认的原版行为（黑盒硬证据）

| 编号 | 结论 | 影响 |
|---|---|---|
| R-01 | 成功响应有两种形态：`{"code":0,"message":"ok","type":"<verb>"}` 与 `{"code":0,"message":"<verb> success","type":"success"}` | 各控制器按接口逐一对齐（见 docs/API.md） |
| R-02 | `listener/add` 即启动；随后 `start` 返回 `already start` | 复刻端已改为 add 内自动启动 |
| R-03 | `listener/add` 的 `EncryptSalt` 缺省为空串，`MaxDNSsize=512` | 帧密钥 = `md5("")`；agent 侧必须接受空盐 |
| R-04 | `listener/list` 每项 **14 个键**：ConnectAddr/DNSDomain/DisconnectTimeout/EncryptSalt/Id/ListenAddr/MaxDNSsize/Mode/OssUrl/PingInterval/PublicDNS/Remark/Status/Vkey | 无 Port/Host/Proxy/Salt/vip |
| R-05 | 空的 `listener/list`、`tunnel/list` 返回 `items: null`（不是 `[]`） | 前端表格需容忍 null |
| R-06 | 错误文案：`connection error`（file/screenshot/install/runner）、`client is close`（terminal）、`not found`（listener/start、download 无监听器）、`already start`、`start error`/`stop error`（tunnel）、`delete success`（幂等删除）、`client ID not found` / `client id not found`（两种大小写都出现过） | 逐字对齐，见 contract_test |
| R-07 | 载荷模板路径：`stageless/<tp>_<arch>`、`dll/<tp>_<arch>`；DNS 用 `stageless/ebpf_<tp>_<arch>`；缺失时报 `open <path>: file does not exist` | `PoTemplatePath` 已按此实现 |
| R-08 | `download/stage`、`download/shellcode` 的监听器模式必须是 tcp/ws，否则 `Stage support TCP/WS only` | 模式门已按此实现 |
| R-09 | `/api/getMenuList`、`/api/getPermCode` 在原版 **404**（SPA 另有回退） | 我们保留为扩展，已登记为偏差 |
| R-10 | `/api/client/add` 在原版返回 **HTTP 500（空 body）**——原版的 bug | 我们保留可用实现，登记为偏差 |
| R-11 | `client/list` 的 `clientCount` 在原版曾出现 `-1`（计数器语义未恢复） | 我们用真实总数，登记为偏差 |
| R-12 | `db/data.db` 的 4 张表与 `c2engine/storage.go` 的建表语句 **逐列一致** | schema 对齐从"反编译推断"升级为"运行时实dump" |
| R-13 | Basic Auth 开启时 `GET /` 返回 401 + `WWW-Authenticate: Basic realm="Authorization Required"`，body `401 Unauthorized` | 复刻端尚未实现该中间件（B5.7） |

---

## 4. 取证纪律

1. **只读优先**：探测脚本只用 GET/列表类 POST；写接口（listener/tunnel/client 增删）只在必要时用，并立刻清理。
2. **不复用原版实例做破坏性实验**（不删插件、不改 conf、不写 DB）。
3. 需要看 DB 时先 `cp db/data.db* /tmp/xxx/` 再读副本，避免锁库。
4. 每次取证后更新 fixtures 并跑契约测试，diff 即"回归"。

---

## 5. 逆向工具链

| 工具 | 位置/用法 | 备注 |
|---|---|---|
| IDA Pro 9.5 | `/home/kali/ida-pro-9.5/`（`ida`、`idat`、`idalib`） | 已在本机；502 MB Go 二进制分析吃内存（本机 7 GB），建议 `idalib` headless + 专项问题 |
| IDA MCP | dsh 已配置 `mcp__ida__*`（open_database / execute_python / reference …） | 适合"按地址取反编译/交叉引用"的定点提问 |
| 黑盒面板 | `scripts/probe-original.sh` | 契约层最便宜的取证方式 |
| pclntab | 待建：解析 ELF 里的 Go pclntab（`runtime.*` 名字仍可读，用户函数名可能被 garble） | 可批量恢复"名字 ↔ 地址"，比 IDA 逐个找快 |

**注意**：仓库里的 `FUN_01xxxxxx` 地址来自 Windows 版 `v_windows_amd64.exe` 的 Ghidra 工程；
本机这份是 **Linux 4.9.3 构建**，地址与 Windows 版不同。跨版本对齐时以"函数名/字符串/常量"
为准，不要直接搬地址。

---

## 6. 下一步（与 ROADMAP 对应）

| 任务 | 用参考环境做什么 |
|---|---|
| B1.1/B1.2 协议层 | 建一个临时 tcp 监听器 + 真 agent（若可得）抓 checkin/任务的真实字节；否则用 IDA 定点看 `FUN_016f3e80` 附近 |
| B2.2 载荷配置块 | 若能拿到官方 `stageless/*_windows_amd64` 模板，直接作为载荷回归样本（当前缺失，需另寻） |
| B5.7 Basic Auth | 对齐 401 头与 body（R-13） |
| U-01/U-02 argv key | IDA 定点：`FUN_018dca00` 的调用方闭包捕获值 |
