# 原生 MCP 服务（MCP）

> 目标：让 AI 客户端（Claude Desktop、dsh 的 MCP 连接器、任何支持 MCP 的 IDE）
> 直接操作这台 C2，不必经过面板 UI。这是本项目"后端原生 MCP"能力的实现说明。

---

## 1. 启动

```bash
# 与服务端读同一份 conf/setting.conf（因此 cwd 要对）
./vshell-server mcp        # stdio：每行一条 JSON-RPC 2.0 消息
```

服务端（Web 面板 + C2 引擎）需要**同时在跑**：MCP 工具通过本机面板 API 执行，
与前端走同一条业务路径（鉴权、参数校验、错误文案、超时都复用控制器）。

## 2. 客户端配置示例

```jsonc
// Claude Desktop / dsh: mcp.json
{
  "mcpServers": {
    "vshell": {
      "command": "/home/kali/Desktop/vshell/vshell-server",
      "args": ["mcp"],
      "cwd": "/home/kali/Desktop/vshell"   // 必须与运行面板的目录一致（读 conf/）
    }
  }
}
```

## 2.1 实测（R64，交付包实例）

在 `~/vshell-release` 上跑通：

```
initialize → serverInfo {name: vshell, version: 4.9.3-replica}，capabilities.tools = true
tools/list → 13 个工具
tools/call dashboard      → 真实 KPI（clientCount=1 / clientOnlineCount=1 / cpu / disk…）
tools/call list_clients   → 客户端条目（含 Arch=amd64、HostName…）
tools/call run_command    → {"output":"kali\nkali\n"}（真的在被控端执行了 id -un && hostname）
```

## 3. 协议实现

| 方法 | 说明 |
|---|---|
| `initialize` | 返回 `protocolVersion: 2024-11-05`、`capabilities.tools`、`serverInfo` |
| `tools/list` | 13 个工具，含 JSON Schema |
| `tools/call` | 返回 `content:[{type:"text",text:"<JSON>"}]`；失败置 `isError: true` |
| `ping` | 空结果 |
| `resources/list` | 6 份只读文档资源（STATUS/ROADMAP/PROTOCOL/API/MCP/REFERENCE） |
| `resources/read` | 读 `vshell://docs/<file>`；白名单外或路径穿越一律拒绝 |

错误码：`-32700` 解析错误、`-32601` 未知方法、`-32602` 参数错误；未知工具走
`tools/call` 的 `isError` 通道（不影响协议层）。

## 4. 工具清单

| 工具 | 参数 | 作用 |
|---|---|---|
| `list_clients` | — | 客户端列表（在线状态/主机/系统/通道/地址） |
| `list_listeners` | — | 监听器列表 |
| `create_listener` | `mode` `vkey` [`host` `port` `remark` `dns_domain` `ping_interval`] | 创建监听器（自动启动） |
| `run_command` | `client_id` `command` | 在目标上执行命令并回显（同步 ≤20s） |
| `list_files` | `client_id` `path` | 目录列表 |
| `read_file` | `client_id` `path` | 读文本文件 |
| `write_file` | `client_id` `path` `content` | 写文本文件（覆盖） |
| `upload_file` | `client_id` `local_path` `remote_dir` | 本机文件 → 目标（multipart → agent 落盘） |
| `download_file` | `client_id` `remote_path` `local_path` | 目标文件 → 本机 |
| `dashboard` | — | 概览（计数、CPU/磁盘、版本、许可） |
| `list_tunnels` | — | 隧道列表 |
| `start_tunnel` | `client_id` `port` `target` [`server_addr`] | 建并启动端口转发（见 PROTOCOL §3.5） |
| `screenshot` | `client_id` `local_path` [`quality`] | 截图存 PNG |

所有工具返回**结构化 JSON 文本**，便于模型直接解析；文件类工具返回保存路径与字节数。

## 5. 鉴权

MCP 进程启动时用配置里的 `web_username`/`web_password` **真正登录一次**面板拿 JWT，
并在内存里缓存 20 小时：

- 为什么不本地自签：跨进程的 JWT 密钥可能不同（`web_jwt_secret` 为空时每个进程
  随机生成），自签 token 会被面板判为无效（表现为 `code:-1` 且 `message` 为空）。
- 该修复由 `mcp/server_test.go:TestMCPLogsInForToken` 覆盖。

## 6. 与 HTTP API 的关系

```
MCP 客户端 ──stdio──▶ vshell-server mcp ──HTTP(Token)──▶ 面板 API ──▶ 引擎 ──▶ agent
```

MCP 只是"另一个客户端"，不新增业务逻辑；因此面板与 MCP 的能力永远一致。
若将来需要**脱离面板**运行（比如纯 CLI 模式），可以把 `mcp/tools.go` 的
`apiJSON/apiRaw` 换成直接调用 `controllers` 的导出函数——接口不变。

## 6.5 资源（resources）

把仓库文档暴露成只读资源，AI 客户端可直接读取项目状态而不必让用户粘贴：

| URI | 内容 |
|---|---|
| `vshell://docs/STATUS.md` | 各模块还原状态（附证据） |
| `vshell://docs/ROADMAP.md` | 里程碑与任务清单 |
| `vshell://docs/PROTOCOL.md` | 线协议 |
| `vshell://docs/API.md` | 面板 API 契约 |
| `vshell://docs/MCP.md` | 本文 |
| `vshell://docs/REFERENCE.md` | 原版黑盒行为与工具链 |

白名单固定（`mcp/resources.go`），`..` 之类路径穿越会被拒绝；docs 目录从 cwd 向上最多找 4 层（便于从 `web/`、`mcp/` 等子目录启动）。

## 7. 已验证

- `mcp/server_test.go`：握手/工具清单/错误码/登录取 token/工具打 API 五组用例
- `scripts/e2e.sh` §7l：真实服务 + 真实 agent 下，`initialize`、
  `tools/call list_clients`（含 agent Vkey）、`resources/list` + `resources/read` 全部通过
