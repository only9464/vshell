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
| `/login` | ✅ 完成 | 用户名/密码 + 回车提交 |
| `/dashboard/index` | 🚧 基础版 | KPI 卡片；待补：运行信息表、流量/趋势、最近上线 |
| `/clientManager/index` | ⬜ | 列表列、操作列、右键菜单、详情抽屉 |
| `/clientManager/terminal` | ⬜ | xterm + WS（沿用已验证的接口） |
| `/clientManager/filemanager` | ⬜ | 目录树/表格、上传、编辑、下载 |
| `/clientManager/screen` | ⬜ | 实时画面 + 质量切换 |
| `/listener/index` | ⬜ | 列表 + 新建/编辑弹窗（模式、Vkey、盐、心跳） |
| `/download/index` | ⬜ | 载荷类型 × 架构、复制命令 |
| `/tunnel/index` | ⬜ | 列表 + 新建/编辑/启停 |
| `/pluginrunner/index` | ⬜ | 插件列表 + 执行 + 输出 |
| `/setting/index` | ⬜ | 设置表单 |
| `/about/index` | ⬜ | 版本信息 |

## 5. 验证方式

```bash
cd web2 && pnpm build
# 临时实例验证（不干扰正在运行的交付包）
mkdir -p /tmp/fe2/{conf,db,web} && ln -sfn $PWD/web2/dist /tmp/fe2/web/dist
# 起服务后：go run ./tools/uiprobe -url http://127.0.0.1:18190/app/#/login -script '…'
```

已用上述方式验证：登录成功 → 跳转 `#/dashboard/index` → 菜单与 KPI 正常渲染。
