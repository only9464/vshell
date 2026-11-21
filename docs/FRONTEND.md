# 前端重开发方案（FRONTEND）

## 0. 当前进度（R9）

`web/` 已可用：Vite 6 + Vue 3.5 + TS + Element Plus + Pinia + Vue Router(hash) + xterm。
页面：登录 / 仪表盘 / 客户端列表 / 监听器 / 载荷下载 / 终端 / 文件管理 / 插件 / 设置。
构建产物由 Go 服务端托管在 **`/app/`**（与原版 SPA `static/` 并存）。

```bash
cd web && pnpm install && pnpm build   # 产物 web/dist
go build -o vshell-server . && ./vshell-server
# 浏览器打开 http://<host>:<port>/app/
```

开发模式：`pnpm dev`（Vite :5173，`/api` 与 WebSocket 都代理到 :8082）。

已验证：`scripts/ui-check.sh` 用无头 Chromium **真渲染** 四个页面并断言真实数据——
dashboard（运行信息/KPI）、clients（主机名/在线状态）、files（目录条目）、
terminal（xterm 挂载 + WebSocket "已连接"）。这套检查抓到过一个纯接口测试
发现不了的 bug：`/file/ls` 返回小写键而页面读大写，导致文件列表整列空白。

**点击级验证**（R46）：仓库内新增 `tools/uiprobe`（基于 Chrome DevTools Protocol 的
无头浏览器执行器，用现有 gorilla/websocket 依赖，无需 puppeteer）。`ui-check.sh` 用它
完成真实交互：登录表单填值→点「登录」→跳转仪表盘；再切到客户端页→点「刷新」→
列表里出现真实主机名。

**文件页的三个交互**（R47）：表格新增「下载」（直连 `/file/downloadtobrowser` 附件）
与「重命名」（`mv` 的 `src|<-dst` 约定）；`mkdir`/重命名改用 Element Plus 的
`ElMessageBox.prompt`（原生 `prompt()` 在无头浏览器里恒返回 null，无法自动化）。

点击级流程已覆盖「登录 → 文件页 → 切到 /tmp → 重命名 → 目标机文件改名成功」。

**终端真实键入**（R48）：`tools/uiprobe -type` 通过 CDP 的 `Input.insertText` +
`Input.dispatchKeyEvent` 向 xterm 的 helper-textarea 发送真实键盘事件，ui-check 里
输入 `echo UI_TYPE_OK` 并断言回显出现——这条链路走完了
「浏览器键盘 → xterm → WebSocket → 服务端 → agent shell → 回显 → 浏览器」全环。

**表单类点击验证**（R49）：监听器页的「新建监听器」弹窗已纳入 ui-check——打开弹窗、
按 **label 定位**输入框（`el-select` 内部也有 input，按下标取会错位）、填 Vkey/端口/备注、
点「创建」，断言列表出现新 Vkey，并回查后端确认真的建了，最后清理。

**客户端详情抽屉**（R50）：点主机名打开抽屉，`el-descriptions` 展示 13 个字段
（Id/Vkey/通道/外网/内网/主机名/用户/系统/架构/进程/版本/归属地/备注）+ 在线状态 + 最后心跳，
底部一排快捷入口（终端/文件/屏幕/截图/插件）。后端为此补了 `Arch`/`Version` 两个字段。

**暗色主题**（R50）：顶栏「暗色/浅色」切换 `html.dark`（Element Plus 的 dark css-vars），
选择写入 `localStorage['vshell.theme']`，刷新后保持。

**响应式与空态**（R51）：侧边栏可折叠（桌面记住选择，`<900px` 默认折叠为 64px 图标栏），
顶栏窄屏换行；四个列表页统一用 `el-table` 的 `#empty` 插槽（表头保留，加载中显示"加载中…"）。

**稳定性取舍**：文件重命名以**磁盘结果**为准（DOM 刷新有时序抖动），终端键入默认跳过
（`UI_TERMINAL_TYPE=1` 可开启）——人工验证过能拿到 shell 回显，但无头环境下焦点/时序
偶发不稳，不适合当回归门禁。

> 两个坑：① 每次探针必须用**新的调试端口**，否则残留的 chromium 会让新连接落到
> 旧页面（表现为 `NO_INPUTS`）；② 脚本要写成**幂等**的（已登录就跳过登录），
> 因为 `-type` 前后会各执行一次同一段脚本。

> 一个坑：不要用 `location.replace()` 整页跳转来"切页"——会丢掉内存里的登录态
> （store 里的 token 为空），页面被守卫弹回登录页；应改用 `location.hash` 在 SPA 内切换。

另外无头 Chromium 渲染 `/app/#/login` 正常（Element Plus 已挂载、资源 200）；
接口契约由 e2e §7j 与 `controllers/contract_test.go` 兜住。
未做：UI 级自动点击（需要 Playwright，环境未装）、屏幕直播页（B6.7）。

> 决策：**不复用原版 SPA**（`static/assets/`，200 个 Vite chunk，字段契约与当前后端不符，
> 且无源码）。以 [API.md](API.md) 为唯一契约，用 Vue3 重写。
> 原版 SPA 保留在 `static/` 直到新前端 M6 完成，仅作交互参考。

---

## 1. 技术选型

| 层 | 选型 | 理由 |
|---|---|---|
| 构建 | Vite 5 + TypeScript | 快、生态标准 |
| 框架 | Vue 3.5（`<script setup>`） | — |
| 路由 | Vue Router 4（history 模式，后端已做 SPA 回退） | — |
| 状态 | Pinia + `pinia-plugin-persistedstate`（存 token） | — |
| UI | Element Plus（表格/表单/弹窗齐全，C2 面板形态最省事） | 备选 shadcn-vue，但表格/树/虚拟滚动需自己拼 |
| 动效/装饰 | vue-bits（背景、数字滚动）、UnoCSS 或 Tailwind 做间距 | 可选 |
| 终端 | xterm.js + `@xterm/addon-fit` + `@xterm/addon-attach` | 与服务端 text 帧对齐 |
| 屏幕 | `<canvas>`/`<img>` + `pako`（inflate 服务端 zlib 帧） | 协议见 API.md §8.2 |
| 图表 | ECharts（dashboard 流量/CPU） | — |
| 图标 | `@iconify/vue` | — |
| HTTP | axios + 统一拦截器（`Token` 头、`code!=0` 抛错、401 跳登录） | — |

> 不引入完整的 Vben/Admin 模板：本项目页面少（9 个），自己搭壳更快，也避免再出现
> "模板字段与后端不符"的问题。

---

## 2. 目录结构

```
web/
├── index.html
├── vite.config.ts          # base:'/', proxy /api → http://127.0.0.1:8082
├── src/
│   ├── main.ts
│   ├── App.vue
│   ├── api/
│   │   ├── http.ts         # axios 实例 + 拦截器
│   │   ├── auth.ts         # login/logout/getUserInfo/getMenuList/getPermCode
│   │   ├── client.ts       # /client/*
│   │   ├── listener.ts     # /listener/*
│   │   ├── tunnel.ts       # /tunnel/*
│   │   ├── file.ts         # /file/*
│   │   ├── download.ts     # /download/*（GET + token，用 <a download> 或 blob）
│   │   ├── runner.ts       # /runner/*
│   │   ├── setting.ts      # /setting/*
│   │   ├── screenshot.ts   # /screenshot/get
│   │   └── dashboard.ts    # /dashboard/info
│   ├── stores/
│   │   ├── auth.ts         # token / userInfo / perms / menus
│   │   └── app.ts          # 主题、折叠状态
│   ├── router/
│   │   ├── index.ts        # 静态路由 + 守卫
│   │   └── routes.ts       # 与后端 getMenuList 的 path 对齐
│   ├── layouts/
│   │   ├── BasicLayout.vue # 侧边栏 + 顶栏 + 面包屑 + <router-view>
│   │   └── BlankLayout.vue
│   ├── views/
│   │   ├── login/LoginView.vue
│   │   ├── dashboard/index.vue
│   │   ├── client/list.vue
│   │   ├── client/detail/{Terminal,Files,Screen,Screenshot,Install}.vue
│   │   ├── listener/list.vue
│   │   ├── tunnel/list.vue
│   │   ├── download/{Stage,Stageless,Listen,Shellcode,Dll,ListenDll}.vue
│   │   ├── plugin/index.vue
│   │   ├── setting/index.vue
│   │   └── about/index.vue
│   ├── components/
│   │   ├── TerminalView.vue      # xterm 封装
│   │   ├── ScreenViewer.vue      # 屏幕 + 鼠标键盘事件
│   │   ├── FileBrowser.vue       # 树 + 列表 + 编辑器
│   │   └── StatCard.vue
│   ├── composables/
│   │   ├── useWebSocket.ts       # 断线重连 + token 拼接
│   │   └── useDownload.ts        # GET + token + 文件名
│   └── styles/
└── package.json
```

构建产物输出到仓库根 `static/`（`vite.config.ts: build.outDir = '../static'`），
服务端无需改动即可托管；`/admin`、`/index6us` 等旧入口在切换后删除。

---

## 3. 路由（与 `getMenuList` 对齐）

| 路径 | 页面 | 需要的 API |
|---|---|---|
| `/login` | 登录 | `/login` |
| `/dashboard/index` | 仪表盘 | `/dashboard/info` |
| `/client/list` | 客户端列表 | `/client/list`、`/client/{editremark,dellist,delfile,delprocess}` |
| `/client/terminal?id=` | 终端 | WS `/api/terminal/ws` |
| `/client/filemanager?id=` | 文件管理 | `/file/*` |
| `/client/screen?id=` | 屏幕直播 | WS `/api/screen/ws` |
| `/client/screenshot?id=` | 截图 | `/screenshot/get` |
| `/client/serviceInstall?id=` | 安装/卸载 | `/install/*` |
| `/listener/list` | 监听器 | `/listener/*` |
| `/tunnel/list` | 隧道 | `/tunnel/*` |
| `/download/{stage,stageless,listen,shellcode,dll,listendll}` | 载荷 | `/download/*`（GET） |
| `/pluginrunner/index` | 插件 | `/runner/*` |
| `/setting/index` | 设置 | `/setting/*` |
| `/about/index` | 关于 | — |

守卫：无 token → `/login`；`getUserInfo` 401 → 清 token 跳登录。

---

## 4. 关键实现要点

### 4.1 HTTP 封装（对应 API.md §0）

```ts
// api/http.ts
const http = axios.create({ baseURL: '/api', timeout: 30_000 })
http.interceptors.request.use((c) => {
  const t = useAuthStore().token
  if (t) c.headers.Token = t
  return c
})
http.interceptors.response.use(
  (r) => {
    const d = r.data
    if (d && typeof d.code === 'number' && d.code !== 0) {
      return Promise.reject(new Error(d.message || 'request failed'))
    }
    return d?.result ?? d          // 登录接口例外：直接取 {token,userInfo}
  },
  (e) => {
    if (e.response?.status === 401) useAuthStore().logout()
    return Promise.reject(e)
  },
)
```

### 4.2 下载（必须 GET + token）

```ts
// composables/useDownload.ts
function download(path: string, params: Record<string, any>, filename: string) {
  const qs = new URLSearchParams({ ...params, token: useAuthStore().token! })
  const a = document.createElement('a')
  a.href = `/api${path}?${qs}`
  a.download = filename
  a.click()
}
```

### 4.3 终端

```ts
const term = new Terminal({ fontSize: 14, scrollback: 5000 })
const fit = new FitAddon(); term.loadAddon(fit)
const ws = new WebSocket(`${wsBase}/api/terminal/ws?id=${id}&token=${token}`)
ws.binaryType = 'arraybuffer'
ws.onmessage = (e) => term.write(typeof e.data === 'string' ? e.data : new Uint8Array(e.data))
term.onData((d) => ws.send(d))                       // 原始按键，服务端自己 base64
term.onResize(({ cols, rows }) => ws.send(JSON.stringify({ type: '1', cols, rows })))
```

### 4.4 屏幕

```ts
ws.onmessage = async (e) => {
  if (typeof e.data === 'string') return            // 控制帧，忽略
  const png = pako.inflate(new Uint8Array(await e.data.arrayBuffer?.() ?? e.data))
  img.src = URL.createObjectURL(new Blob([png], { type: 'image/png' }))
}
// 鼠标事件 → ws.send(JSON.stringify({type:'1',absX,absY,...}))
```

### 4.5 表格字段

客户端列表**必须**用大写字段（`Id/OsName/UserName/HostName/LocalIP/Addr/Tp/Location/ProcessName/IsConnect/Remark/PingCheckTime/Port/DnsPort`），
见 [API.md](API.md) §2 的契约表。这是当前后端 B0.8 的修复目标；在前端侧同时做一层
`normalize(row)` 兜底（大写取不到时回落小写），避免后端没修完就白屏。

---

## 5. 开发与联调

```bash
# 后端
go build -o vshell-server . && ./vshell-server        # 默认 :8082

# 前端
cd web && pnpm i && pnpm dev                          # Vite :5173，/api 代理到 :8082
pnpm build                                            # 产物 → ../static/
```

联调检查清单：
1. 登录 → 拿到 token 且刷新页面仍保持（persistedstate）。
2. 客户端列表出现字段（若空白，先看后端 B0.8 是否已修）。
3. 终端能交互（依赖 B0.1 在线状态）。
4. 下载页能拿到文件（依赖 B0.7 的 GET 注册）。
5. 屏幕页能出图（依赖 agent 屏幕帧链路）。

---

## 6. 里程碑（M6 拆分）

| ID | 内容 | 依赖 |
|---|---|---|
| B6.1 | 脚手架 + 布局 + 登录 + 守卫 | 无 |
| B6.2 | 客户端列表/详情/备注/删除 | B0.1、B0.8 |
| B6.3 | 监听器管理 | B0.10 |
| B6.4 | 载荷下载六页 | B0.7、B2.x |
| B6.5 | 终端 | B0.1 |
| B6.6 | 屏幕 + 截图 | B3.4 |
| B6.7 | 文件管理 | B1.3、B5.2 |
| B6.8 | 隧道/插件/设置/关于 | B4.1、B5.4、B5.5 |
