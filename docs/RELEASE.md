# 部署与运维

本文面向"把这套复刻端跑起来并日常使用"的场景。开发细节见 [DEV.md](DEV.md)，
接口契约见 [API.md](API.md)，现状与差距见 [STATUS.md](STATUS.md) / [GAPS.md](GAPS.md)。

## 载荷模板（下载功能的前置条件）

`/api/download/*` 需要工作目录下有模板文件 `stageless/<tp>_<arch>`（例如
`stageless/http_linux_amd64`）；服务端会在这份模板尾部追加配置块，agent 启动时
从自身尾部读取配置并连接对应监听器。

用仓库脚本一键生成（默认把当前平台全部传输都生成）：

```bash
TARGETS="linux/amd64 windows/amd64" TPS="http tcp ws kcp dns" \
  ./scripts/make-templates.sh /path/to/release-dir
```

生成后目录结构：

```
release-dir/
├── vshell-server
├── conf/setting.conf
├── stageless/{http,tcp,ws,kcp,dns}_linux_amd64
└── dll/ listen/            # 与 stageless 同名（占位复用）
```

缺模板时的表现：下载接口返回
`{"code":-1,"message":"open stageless/http_linux_amd64: file does not exist"}`
（与原版逐字一致），面板会提示下载失败。

## 1. 构建

```bash
# 服务端（内含面板静态资源，前端构建产物在 web/dist）
(cd web && pnpm install && pnpm build)
go build -o vshell-server .

# agent（独立子模块）
(cd agent && go build -o agent .)
```

> 前端构建产物会随服务端一起被嵌入/托管在 `/app/`（见 `router/router.go`），
> 因此部署时**只需要带上 `vshell-server`、`web/dist`、`static/`、`conf/`、`db/`**。

## 2. 首次启动

```bash
mkdir -p conf db
cat > conf/setting.conf <<'CONF'
master_type=web
web_port=8082
web_ip=0.0.0.0
web_title=VShell
web_basic_auth=false
web_jwt_secret=            # 留空则首次启动随机生成并固定
web_username=admin
web_password=ChangeMe123
CONF
./vshell-server
```

浏览器打开 `http://<host>:8082/` → 自动跳转 `/app/`，用上面的账号登录。

要点：

- `web_jwt_secret` **留空更安全**（首次生成后固定，重启保持）；填了就把 JWT 固定成该值。
- 面板地址是 `/app/`（hash 路由），`/` 会 302 过去。
- 若开了 `web_basic_auth=true`，访问 `/app/` 会多一层 Basic Auth。

## 3. 上线一个客户端

1. 面板「监听器」→ 新建：模式 `http/kcp/websocket/https/dns`，填端口、Vkey、加密盐。
2. 面板「载荷下载」→ 选监听器与架构 → 下载 stageless，或用「复制拉取命令」在目标机上
   `curl` 下来。
3. 目标机执行载荷，面板「客户端」页出现新主机（绿色"在线"）。

```bash
# 例：本地自测（agent 与服务端同机）
./agent -server http://127.0.0.1:8082 -vkey <Vkey> -salt <盐>
```

## 4. 常用运维动作

| 需求 | 做法 |
|---|---|
| 看服务端日志 | 前台运行即可；后台运行时重定向到文件 |
| 改设置 | 面板「设置」，或直接改 `conf/setting.conf` 后重启 |
| 备份 | 备份 `conf/`（配置）与 `db/`（SQLite + `tunnels.json`） |
| 升级 | 替换 `vshell-server`、`web/dist`、`static/`，保留 `conf/` 与 `db/` |
| 健康检查 | `curl -sk https://127.0.0.1:<port>/health`（HTTP 监听器） |
| MCP 接入 | `./vshell-server mcp`，见 [MCP.md](MCP.md) |

## 4.1 插件怎么用

把要执行的文件放进 `plugins/`（支持 `.elf` / `.so` / `.dylib`），面板「插件」页会列出它们，
点「执行」后由**目标机**下载运行并回显输出。

交付包内置了一个示例 `sysinfo.elf`（源码在 `plugins/sysinfo/`）：

```bash
(cd plugins/sysinfo && go build -ldflags="-s -w" -o ../../plugins/sysinfo.elf .)
```

## 5. 验证部署是否正常

```bash
go test ./...          # 单元/集成测试
./scripts/e2e.sh       # 端到端：四条传输 + 面板契约 + 隧道 + 下载 + MCP
./scripts/ui-check.sh  # 前端：11 个页面渲染 + 点击级流程（需要 chromium）
```

三个脚本都是自包含的（起临时服务端/agent，用完清理），可以直接在部署机上跑。

## 6. 已知限制

- **官方 agent 互通**：帧层/认证/任务记录已逐字节对齐，但官方 agent 注册 `main` 通道
  仍未打通（[GAPS A-1](GAPS.md)）。复刻端使用**自己的 agent**（`agent/`）功能完整。
- **未实现**：需要第三方凭证的通知（企业微信/钉钉）与 OSS 上传/监听；见 [GAPS §F](GAPS.md)。
- **平台**：只在 Linux 上验证；Windows/macOS 专有功能未适配（按需求约定）。
