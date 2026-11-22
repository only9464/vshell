# 复刻 4.9.3 前端（新方案）

> 依据：本机 `http://127.0.0.1:39082` 上运行的 **官方 VShell 4.9.3**。
> 交付包里原来的 `web/`（Element Plus）保留可用；新前端在 **`web2/`**，按 4.9.3 的
> 技术栈与结构重写，逐页对齐后再切换为默认。

## 1. 4.9.3 用的技术栈（已在 bundle 中核实）

| 项 | 证据 |
|---|---|
| Vue 3 + Vite | `index.html` 里 `type="module"` 的 `/assets/vr0UQxwYG.js`、`/app.config.js?v=2.10.1-…` |
| **Ant Design Vue** | bundle 内出现 `"ant-design-vue"`、类名 `ant-layout`、`ant-menu-item`、`ant-modal`、`ant-input`、`ant-picker` |
| **UnoCSS** | bundle 内出现 `"unocss"` |
| **@vben**（vben-admin 5.x） | bundle 内出现 `"@vben"`、`"vben"`；布局类名 `ant-layout-auto-collapse-tabs` |
| Tailwind CSS | 与 UnoCSS 并存（presetUno 兼容 Tailwind 写法） |
| 暗色主题 | `<html data-theme="dark">` + `.app-loading` 的 dark 样式 |
| 本地存储键 | `TOKEN__` / `LOCALE__` / `USER__INFO__` / `ROLES__KEY__` / `__APP__DARK__MODE__` |
| 配置 | `/app.config.js`：`VITE_GLOB_API_URL=/api`、`VITE_GLOB_UPLOAD_URL=/api/file/upload`、标题「管理平台」 |
| 登录后首页 | `/api/getUserInfo` 返回 `homePath: "/dashboard/index"` |
| 菜单来源 | `/api/getMenuList` 在 4.9.3 上是 **404** → 路由由前端静态定义 |

## 2. 路由表（从 bundle 提取）

```
/about        /client      /clientManager   /dashboard    /download
/filemanager  /listener    /login           /pluginrunner /screen
/screenshot   /serviceInstall  /setting     /terminal     /tunnel
```

组件名：`clientManager`、`ClientManagerIndex`、`ClientManagerTab`、`screenshotIndex`、
`screenshotTab`、`terminalIndex`、`terminalTab`、`settingIndex`、`pluginrunner`。

## 3. 新前端结构（`web2/`）

```
web2/
├── index.html            # 与 4.9.3 一致：lang=zh, id=htmlRoot
├── vite.config.ts        # base=/app/，构建到 dist/
├── uno.config.ts         # presetUno + attributify + icons
├── tailwind.config.js    # darkMode: [class, [data-theme="dark"]]
├── src/
│   ├── main.ts           # Vue3 + Pinia + Ant Design Vue + uno.css
│   ├── App.vue           # 恢复 data-theme
│   ├── api/http.ts       # /api 前缀、Token 头、code!=0 统一报错、下载直链
│   ├── router/index.ts   # 菜单表 + 路由表（按 4.9.3 路径组织）
│   ├── stores/auth.ts
│   ├── layout/BasicLayout.vue   # vben 风格：深色 Sider + Header + 内容区
│   └── views/            # 逐页移植（见 §4）
```

构建产物由 Go 服务端在 `/app/` 托管（`router/router.go` 读 `web/dist`；
切换时把 `web2/dist` 放到该位置，或改指向）。

## 4. 逐页移植清单

| 页面（4.9.3 路径） | 状态 | 说明 |
|---|---|---|
| `/login` | ✅ 完成 | vben 风格左右分栏（左侧品牌渐变栏 + 右侧表单卡）；占位文案 请输入账号 / 请输入密码；记住我 / 忘记密码?；右上角主题切换；窄屏自动隐藏品牌栏 |
| `/dashboard/index` | ✅ 完成 | KPI 卡片 + 负载趋势（CPU/磁盘双 SVG，4s 采样）+ 运行信息 + 最近上线客户端 |
| `/clientManager/index` | ✅ 完成 | 12 列表格 + 多选（批量删除）+ 分页 + 操作列（带图标）+ 详情抽屉 + **右键菜单**（命令控制/文件管理/查看详情/编辑备注/删除主机） |
| `/clientManager/terminal` | ✅ 完成 | xterm + WS（实测"已连接"），字号/清屏/复制/重连 |
| `/clientManager/files` | ✅ 完成 | **左侧文件树**（懒加载、点击目录切换、默认展开根）+ 右侧目录列表（真实数据）+ 按类型图标 + 进入/上级/编辑/上传/下载/重命名/新建文件夹/删除 + **右键菜单**（进入目录或编辑 / 下载 / 重命名 / 拷贝路径 / 删除文件） |
| ~~屏幕监控~~ | ❌ 按要求删除 | 当前环境无图形会话，功能整体移除（前后端菜单/路由/页面均删，后端 `/api/screen/ws` 保留不再被 UI 使用） |
| `/listener/index` | ✅ 完成 | 列表 + 多选（批量删除）+ 新建/编辑弹窗（模式、Vkey、地址、端口、心跳、盐、备注）+ 启停/删除 |
| `/download/index` | ✅ 完成 | 选监听器 + 8 架构 + 是否UPX压缩 + **使用代理/连接代理** + stageless 下载 + **一键上线命令查看 / Windows 上线命令 / Linux 上线命令**；独立 listen 载荷（连接方式/回连地址/端口/**加密key明文内容**/**加密key加密模式**） |
| `/tunnel/index` | ✅ 完成 | 列表 + 新建/编辑/启停/删除；协议下拉用 4.9.3 文案（TCP代理 / UDP代理 / Socks5隧道 / HTTP隧道），**目前只有 TCP代理 可用**，其余置灰标注"未实现" |
| `/pluginrunner/index` | ✅ 完成 | 插件筛选、执行、输出复制/清空 |
| `/setting/index` | ✅ 完成 | 企业微信/钉钉三个字段 + 保存 |
| `/about/index` | ✅ 完成 | 版本、许可到期、Web 端口、客户端上限 |

## 3.1 文案取自 4.9.3 的 locale 表

从 `static/assets/vB2dXDAOV.js`（4.9.3 前端打包产物）里提取到的原始文案，
新前端已按此对齐：

| 位置 | 4.9.3 文案 |
|---|---|
| 菜单 | 工作台 / 客户端列表 / 监听列表 / 客户端生成 / 隧道代理 / 插件运行 / 系统设置 / 关于 |
| 主题切换 | **亮色主题 / 黑暗主题** |
| 工作台 KPI | **客户端总数 / 在线客户端 / 监听总数 / 在线监听数** |
| 页面名 | 命令控制（终端）、文件管理、屏幕监控（已删）、屏幕截屏（已删） |
| 通用动作 | 确认 / 取消 / 保存 / 删除 / 重置 / 搜索 / 查询 / 刷新 / 返回 / 加载中… |

## 3.2 列名/按钮对照（全部取自 4.9.3 bundle 的 label/title）

| 页面 | 4.9.3 原文 | 已对齐 |
|---|---|---|
| 客户端列表 | 客户端 ID / 客户端状态 / 主机名 / 用户 / 系统类型 / 外网IP / 内网IP / 归属地 / 进程名 / 备注 / 操作 | ✅ |
| 客户端操作 | 编辑 / 删除主机 | ✅ |
| 监听列表 | 监听方式 / 监听地址 / 流量加密盐 / 备注 / 操作；按钮 新增监听、启用/停用、编辑、删除 | ✅ |
| 监听确认 | 是否确认删除监听？ | ✅ |
| 隧道代理 | 服务端端口 / 目标 (IP:端口) / 隧道状态；按钮 新增隧道代理、编辑隧道代理、是否确认删除隧道代理？ | ✅ |
| 客户端生成 | 是否UPX压缩 / 连接方式 / 流量加密盐 / 上线命令查看 / 正向客户端 | ✅ |
| 文件管理 | 文件名 / 修改时间；按钮 新建文件夹、上传插件、重命名、删除文件 | ✅ |

## 3.2b 登录页（对齐 4.9.3 的 vben 登录布局）

4.9.3 登录相关文案（bundle 原文）：`请输入账号`、`请输入密码`、`记住我`、`忘记密码?`、
`其他登录方式`、`登录成功 / 欢迎回来`。新前端据此实现：左侧渐变品牌栏（VS logo +
平台名 + 能力说明），右侧表单（账号/密码/记住我/忘记密码?/登录按钮），右上角主题切换；
<=900px 时隐藏品牌栏只留表单。ui-check 断言 `请输入账号/请输入密码/记住我` 存在。

## 3.3 图标（与 4.9.3 同款 iconify 集合）

4.9.3 的仪表盘用 iconify：`flat-ui:pc`、`flat-color-icons:headset`、`flat-ui:responsive`、
`flat-ui:magic`。新前端通过 UnoCSS `presetIcons` + `@iconify-json/flat-ui`、
`@iconify-json/flat-color-icons` 使用**同一套图标**（构建时内联成 SVG data URI，无运行时请求）：

| KPI 卡 | 图标 |
|---|---|
| 客户端总数 | `i-flat-ui:pc` |
| 监听总数 | `i-flat-color-icons:headset` |
| CPU | `i-flat-ui:responsive` |
| 版本 | `i-flat-ui:magic` |

### 侧栏菜单图标（R80）

4.9.3 bundle 里出现的 iconify 图标共 61 个，菜单用到的对不上语义的部分按同集合选择：

| 菜单 | 图标 |
|---|---|
| 工作台 | `ant-design:home-outlined` |
| 客户端列表 | `fa6-solid:desktop` |
| 监听列表 | `fa6-solid:headphones` |
| 客户端生成 | `fa6-solid:download` |
| 隧道代理 | `ant-design:swap-outlined` |
| 插件运行 | `grommet-icons:services` |
| 系统设置 | `ion:settings-outline` |
| 关于 | `grommet-icons:action` |

> 注意：图标类名是**动态拼接**（`i-${m.icon}`），UnoCSS 静态扫描看不到，
> 必须写进 `safelist`，否则不会生成 SVG。另外 presetIcons 用的是 `mask-image`
> 而不是 `background-image`——验证时要查 `maskImage`。

### 目录专属图标（R84）

目录按名字取 `folder-*` 图标（没有匹配的回落 `folder-base`）：
`src / .git / dist / test / docs / config / api / lib / public / scripts / home / log /
backup / database / docker / kubernetes / vue / angular / python / go / rust / java /
theme / images / font / download / desktop / video / trash`。

图标名已用脚本对集合与 safelist 双向校验（29 个 folder-* 全部存在且已入 safelist）。
实测：根目录下 `home/` 显示 `folder-home`，其余无匹配的显示 `folder-base`。

### 文件树（R83）

`FilesView` 左侧是**自实现的懒加载目录树**（没用 antd Tree：试过 `a-tree` 的
`load-data`/`@select`，点击标题偶发不触发，改为拍平渲染更好控制）：

- `dirs: Record<path, string[]>` 缓存每个目录的子目录，`openDirs` 记录展开状态
- 拍平成一维数组渲染（`depth` 控制缩进），避免递归组件
- 点目录名 → 右表切到该目录；点 ▸/▾ → 懒加载并展开
- 默认展开根 `/`
- `load()` 现在**先更新 path 再发请求**：即便列目录失败，界面也停在目标目录

实测：点击树里的 `etc` → 路径变 `/etc`、表格加载 314 项（含 `passwd`）；
ui-check 增加「文件树点击切换」断言。

### 文件列表按类型显示图标（R81）

文件管理页对每个条目按扩展名显示 **material-icon-theme** 图标（与 Material Icons
浏览器扩展同源的图标集，`@iconify-json/material-icon-theme`，共 1181 个图标）：

| 类型 | 图标 |
|---|---|
| 目录 | `folder-base` |
| 代码 | `typescript / javascript / vue / python / go / rust / java / c / cpp / php / ruby …` |
| 文本 | `document / markdown / log / json / yaml / toml / settings` |
| 媒体 | `image / svg / audio / video / pdf` |
| 归档 | `zip` |
| 其他 | `database / lock / key / certificate / console`，未知后缀回落 `document` |

**特例文件名**（R82）：按名字优先匹配，再按扩展名——

| 文件名 | 图标 |
|---|---|
| `Dockerfile` / `docker-compose.yml` | `docker` |
| `Makefile` / `CMakeLists.txt` | `makefile` / `cmake` |
| `.gitignore` / `.gitattributes` | `git` |
| `package.json` / `pnpm-lock.yaml` / `yarn.lock` | `npm` |
| `tsconfig.json` / `vite.config.*` / `webpack.config.js` | `tsconfig` / `vite` / `webpack` |
| `go.mod` / `Cargo.toml` / `requirements.txt` / `pyproject.toml` | `go` / `rust` / `python` |
| `README.md` / `LICENSE` / `CHANGELOG.md` | `readme` / `license` / `changelog` |
| `nginx.conf` / `Jenkinsfile` / `.gitlab-ci.yml` | `nginx` / `jenkins` / `gitlab` |
| `pom.xml` / `build.gradle` / `Chart.yaml` | `maven` / `gradle` / `helm` |

映射表里的图标名已用脚本对 `material-icon-theme` 集合与 UnoCSS safelist 双向校验（59 个图标全部存在且已列入 safelist）。

实测：`/tmp` 下 `demo.go → go`、`demo.md → markdown`、`demo.py → python`、`demo.zip → zip`，
633 个条目全部渲染出图标。

> 踩坑：pnpm 下 UnoCSS **自动加载**该集合会失败（`failed to load icon`），
> 需要在 `presetIcons({ collections: { 'material-icon-theme': icons.json } })` 里
> **显式注入**；另外 `material-icon-theme` 没有 `folder`/`file`/`excel` 这几个名字，
> 通用目录用 `folder-base`、通用文件用 `document`。

## 4.0 布局对齐 4.9.3

| 元素 | 实现 |
|---|---|
| 侧栏 | 深色 `#001529`（实测 rgb(0,21,41)）、宽 210px、折叠 64px、VS 圆角 logo |
| 顶栏 | 折叠按钮 / 面包屑（首页 › 当前页）/ 主题切换 / 用户菜单，高 56px |
| **页签栏** | vben 风格多标签：按访问顺序累积、可单个关闭、当前页高亮，存 `sessionStorage['vshell.tabs']` |
| 暗色 | `<html data-theme="dark">` + 全局变量覆盖（body/layout/card/table/header） |
| 按钮文案 | 关闭 antd 中文按钮自动空格（"清 屏"→"清屏"） |

## 4.1 接口约定（重要）

4.9.3 的业务接口**几乎都是 POST**（`client/list`、`listener/list`、`file/ls`…），
只有少数是 GET（`/dashboard/info`、`/setting/get`、`/runner/list`、`/screenshot/get`）。
新前端里统一用 `post()`，只有这些 GET 接口用 `get()`——踩过一次 404。

## 5. 验证方式

```bash
cd web2 && pnpm build
# 临时实例验证（不干扰正在运行的交付包）
mkdir -p /tmp/fe2/{conf,db,web} && ln -sfn $PWD/web2/dist /tmp/fe2/web/dist
# 起服务后：go run ./tools/uiprobe -url http://127.0.0.1:18190/app/#/login -script '…'
```

已用上述方式验证：登录成功 → 跳转 `#/dashboard/index` → 菜单与 KPI 正常渲染。
