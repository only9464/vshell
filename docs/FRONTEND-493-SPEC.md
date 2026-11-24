# 4.9.3 前端规格（自动提取）

> 由 `scripts/extract-fe-spec.py` 从 `static/assets/*.js` 提取后整理；本文件入库，作为逐页还原的对照表。

## 1. 菜单与路由（按 orderNo）

| orderNo | path | 图标 | 标题 | 子路由 |
|---|---|---|---|---|
| 10 | `/dashboard` | `fa-solid:tachometer-alt` | **仪表盘** | `index` 仪表盘 |
| 11 | `/client` | `fa-solid:desktop` | **客户端管理** | `list` 客户端管理 |
| 12 | `/listener` | `fa-solid:headphones` | **监听管理** | `list` 监听管理, `:id` 监听管理 |
| 13 | `/download` | `fa-solid:download` | **客户端生成** | `index` 客户端生成, `stage` Stage, `stageless` Stageless, `listen` 正向客户端, `ebpf` ebpf正向客户端 |
| 14 | `/terminal` | `fa-terminal` | **命令控制** | `index` 命令控制, `:id` 命令控制 |
| 15 | `/filemanager` | `grommet-icons:folder` | **文件管理** | `index` 文件管理, `:id` 文件管理 |
| 16 | `/screenshot` | `fa-image` | **屏幕截屏** | `index` 屏幕截屏, `:id` 屏幕截屏 |
| 17 | `/serviceInstall` | `grommet-icons:services` | **开机启动** | `index` 开机启动, `:id` 开机启动 |
| 18 | `/pluginrunner` | `grommet-icons:action` | **插件运行** | `index` 插件运行, `:id` 插件运行 |
| 19 | `/tunnel` | `fa-solid:globe` | **隧道代理** | `list` 隧道代理, `:id` 隧道代理 |
| 20 | `/setting` | `fa6-solid:gear` | **系统设置** | `index` 系统设置 |
| 21 | `/screen` | `fa-image` | **屏幕监控** | `index` 屏幕监控, `:quality/:id` 屏幕监控 |
| 22 | `/clientManager` | `grommet-icons:drag` | **客户端管理** | `index` 客户端管理, `:id` 客户端管理 |
| 23 | `/about` | `` | **关于** | `index` 关于 |

> 说明：`/screenshot`、`/screen` 按需求不做；`/serviceInstall`（开机启动）未实现；
> `/clientManager` 是「主机管理」入口，不进菜单（对应行内按钮与右键菜单的跳转目标）。

## 2. 页面 chunk 明细

### `vPNC64JSn.js`（18876 B）
- **表单**：客户端、清晰度
- **接口**：`/screenshot/get`

### `vBu4f7JWt.js`（14574 B）
- **列**：Mode`mode`
- **菜单/操作**：编辑 `ant-design:edit-outlined`、重命名 `ant-design:font-size-outlined`、修改时间 `ant-design:field-time`、拷贝路径 `ant-design:node-index`、删除 `ant-design:delete-outlined`、下载至OSS `ant-design:cloud-upload-outlined`、下载 `ant-design:download-outlined`、编辑 `ant-design:edit-outlined`、重命名 `ant-design:font-size-outlined`、删除 `ant-design:delete-outlined`

### `vCwa8r0tL.js`（7023 B）
- **菜单/操作**：主机管理 `grommet-icons:drag`、routes.tunnel.tunnel `fa-solid:globe`、删除文件 `grommet-icons:trash`、删除主机 `grommet-icons:close`

### `vBWgKrY3R.js`（5030 B）
- **菜单/操作**：主机管理 `grommet-icons:drag`、routes.tunnel.tunnel `fa-solid:globe`、删除文件 `grommet-icons:trash`、删除主机 `grommet-icons:close`、主机管理 `grommet-icons:drag`、删除文件 `grommet-icons:trash`、删除主机 `grommet-icons:close`

### `vn-sfWjBP.js`（3630 B）
- **表单**：客户端、清晰度

### `vD6-3nvsI.js`（3594 B）
- **列**：ID`Id`、备注`Remark`、模式`Mode`、监听地址`ListenAddr`、外网连接地址`ConnectAddr`、超时心跳次数（次）`DisconnectTimeout`、心跳间隔（秒）`PingInterval`

### `vbw5Ta7i0.js`（3365 B）
- **表单**：客户端类型、监听方式、监听地址、监听端口、Vkey、流量加密盐、是否UPX压缩

### `vD-pDrbd4.js`（3280 B）
- **表单**：选择客户端、选择插件、参数

### `vf5gbhprs.js`（3264 B）
- **表单**：客户端类型、监听方式、监听地址、监听端口、Vkey、流量加密盐、是否UPX压缩

### `vCmx9UE-E.js`（2921 B）
- **表单**：客户端、选择插件、参数

### `vCcgNpaMO.js`（2903 B）
- **表单**：客户端类型、选择监听器、连接代理、是否UPX压缩

### `vB-I98sLM.js`（2784 B）
- **表单**：客户端类型、选择监听器、连接代理、是否UPX压缩

### `viLthP3eG.js`（2696 B）
- **列**：ID`Id`、备注`Remark`、连接方式`Tp`、外网IP`Addr`、归属地`Location`、内网IP`LocalIP`、用户`UserName`、主机名`HostName`、系统类型`OsName`、进程名`ProcessName`
- **表单**：Vkey、流量加密盐

### `vCpjgSZNr.js`（2540 B）
- **表单**：客户端类型、监听方式、Vkey、流量加密盐、是否UPX压缩

### `vCTMULPY8.js`（2408 B）
- **列**：ID`Id`、客户端 ID`ClientId`、备注`Remark`、模式`Mode`、端口`Port`、目标 (IP:端口)`Target`、用户名`Username`、密码`Password`
- **表单**：选择客户端
- **接口**：`/tunnel/add`、`/tunnel/del`、`/tunnel/dellist`、`/tunnel/edit`、`/tunnel/editremark`、`/tunnel/list`、`/tunnel/start`、`/tunnel/stop`

### `v73CBtpUF.js`（2269 B）
- **表单**：客户端类型、选择监听器、格式

### `vXAwImGQr.js`（2259 B）
- **表单**：客户端类型、选择监听器

### `vtg7DcMBU.js`（2253 B）
- **表单**：钉钉 token、钉钉 key_word、微信 wx_key
- **接口**：`/setting/edit`、`/setting/get`

### `vBG86JV9o.js`（2174 B）
- **表单**：客户端、安装服务名称、服务描述
- **接口**：`/install/install`、`/install/remove`

### `vC5gqvPSL.js`（2116 B）
- **表单**：选择客户端

## 3. 动态分析：真实 UI（R144）

打包产物在本机能直接跑起来：`scripts/serve-493.py` 把 `static/` 本地伺服，并把
`/api/*` 反向代理到真实实例 `:39082`（自动带 Basic 认证）。这样 Chromium 里就能
渲染 4.9.3 的真实界面，配合 `tools/uiprobe -shot` 逐页截图比对。

```bash
python3 scripts/serve-493.py static 18990 http://127.0.0.1:39082 &
go run ./tools/uiprobe -url http://127.0.0.1:18990/ -wait 8s -shot /tmp/x.png -script "..."
```

### 3.1 布局（与当前实现的重大差异）

真实 4.9.3 是 **顶部导航**布局，不是左侧栏：

```
┌─ header ───────────────────────────────────────────────────────────┐
│ [logo] 管理平台 │ 仪表盘 客户端管理 监听管理 客户端生成 插件运行 隧道代理 系统设置 关于 │ 明暗 语言 admin ⚙ ⛶ │
├─ tabs ─────────────────────────────────────────────────────────────┤
│ [仪表盘 ✕] [客户端管理 ✕]                                    ↻ ⋯ │
├────────────────────────────────────────────────────────────────────┤
│ 内容区                                                              │
└────────────────────────────────────────────────────────────────────┘
```

- 顶部菜单只有 8 项：仪表盘、客户端管理、监听管理、客户端生成、插件运行、隧道代理、
  系统设置、关于（**命令控制/文件管理不在菜单里**，它们从客户端行内进入）
- 页签可关闭（✕），右侧有 刷新 / 更多 两个图标
- 登录页是**居中卡片**（标题「登录」+ 两个输入框 + 蓝色按钮），不是左右分栏

### 3.2 仪表盘

- 4 个 KPI 卡片（监听总数 / 在线监听数 / 客户端总数 / 在线客户端，数字在左、图标在右）✅ 与我们一致
- 下面是**左右两栏**：左「配置信息」、右「系统信息」（我们顺序反了）
- 系统信息 = 4 个进度条 + TCP/UDP + 流出/流入带宽；配置信息 = 8 行键值
- **没有**「负载趋势」「最近上线客户端」这两块（我们多加了）

### 3.3 客户端生成 = 7 个页签（不是 4 个区块）

| 页签 | 字段 | 按钮 |
|---|---|---|
| stage | 客户端类型、选择监听器 | 生成下载 |
| shellcode | 客户端类型、选择监听器、格式 | 生成下载 |
| stageless | 客户端类型、选择监听器、连接代理、是否UPX压缩 | 生成下载 |
| dll stageless | 同上 | **下载Loader**、生成下载 |
| listen | 客户端类型、监听方式、监听地址、监听端口、Vkey、流量加密盐、是否UPX压缩 | 生成下载 |
| dll listen | 同上 | **下载Loader**、生成下载 |
| ebpf listen | 客户端类型、监听方式、Vkey、流量加密盐、是否UPX压缩 | 生成下载 |

每个页签卡片顶部有「标题 + 一句话说明」，例如 Stage 是「Stage 反向客户端 / 小体积客户端，反连模式上线」。

### 3.4 客户端列表

见截图：工具栏「搜索 / 批量选择」「正向连接 + 刷新/字号/设置图标」；表格列与我们已对齐的
13 列一致；备注列有 ✎ 图标（行内编辑）；空态是文件夹图标 +「暂无数据」。

### 3.5 路由（已按原版改造）

复刻端的路由现在与 4.9.3 **逐条一致**（旧路径保留重定向）：

| 4.9.3 | 复刻端 | 说明 |
|---|---|---|
| `/dashboard/index` | ✅ | 仪表盘 |
| `/client/list` | ✅ | 客户端列表（`/clientManager/index` 重定向到此） |
| `/clientManager/index`、`/clientManager/:id` | ✅ | 主机管理 |
| `/listener/list`、`/listener/:id` | ✅ | 监听（`/listener/index` 重定向） |
| `/download/index|stage|stageless|listen|ebpf` | ✅ | 5 个子路由（同一页按页签区分） |
| `/terminal/index`、`/terminal/:id` | ✅ | 命令控制 |
| `/filemanager/index`、`/filemanager/:id` | ✅ | 文件管理 |
| `/pluginrunner/index`、`/pluginrunner/:id` | ✅ | 插件运行 |
| `/tunnel/list`、`/tunnel/:id` | ✅ | 隧道（`/tunnel/index` 重定向） |
| `/setting/index` | ✅ | 系统设置 |
| `/about/index` | ✅ | 关于 |
| `/login`、`/loginSSO/:token` | ✅ | 登录 / SSO 入口 |
| `/screenshot/*`、`/screen/*` | — | 按需求不做 |
| `/serviceInstall/*` | — | 未实现 |
| `/error-log`、`/redirect/*` | — | 未实现 |

实测 9 条一级/子路由全部渲染正确。

### 3.6 文件管理页（真实 UI）

- 左侧「文件树」面板：搜索框 + ⋮ 菜单 + 树/空态（暂无数据）
- 右侧工具栏：`↻`（刷新）`🏠`（根目录）`↑`（上级）`↻`（重载）+ 路径输入 + **Open** +
  **新建文件夹** + **新建文件** + **wget文件** + **上传文件**
- 表格列头是英文：**Name / Time / Mode / Size**（不是中文）

> 我们的实现：按钮文案是「打开 / 上级 / Wget下载文件 / 上传插件」，列头是中文
> 「文件名/大小/修改时间/权限」——按原版应改成上表。
