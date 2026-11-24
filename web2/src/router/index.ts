import { createRouter, createWebHashHistory } from 'vue-router'
import { TOKEN_KEY } from '../api/http'

// 路由与菜单**完全对齐 4.9.3**（来自 static/assets/vr0UQxwYG.js 的路由定义 +
// vB2dXDAOV.js 的 locale，orderNo/icon/title 均照抄）：
//
//   10 /dashboard      仪表盘          fa-solid:tachometer-alt   → /dashboard/index
//   11 /client         客户端管理      fa-solid:desktop          → /client/list
//   12 /listener       监听管理        fa-solid:headphones       → /listener/list | /listener/:id
//   13 /download       客户端生成      fa-solid:download         → index|stage|stageless|listen|ebpf
//   14 /terminal       命令控制        fa-terminal               → /terminal/index | /terminal/:id
//   15 /filemanager    文件管理        grommet-icons:folder      → /filemanager/index | /filemanager/:id
//   18 /pluginrunner   插件运行        grommet-icons:action      → /pluginrunner/index | /pluginrunner/:id
//   19 /tunnel         隧道代理        fa-solid:globe            → /tunnel/list | /tunnel/:id
//   20 /setting        系统设置        fa6-solid:gear            → /setting/index
//   22 /clientManager  主机管理        grommet-icons:drag        → /clientManager/index | /clientManager/:id
//   23 /about          关于            (无图标)                  → /about/index
//
// 另外 4.9.3 还有 /screenshot、/screen、/serviceInstall 三组（按需求不做/未实现），
// 以及 /loginSSO/:token、/redirect/...、/error-log。

export const menus = [
  { path: '/dashboard/index', name: 'Dashboard', title: '仪表盘', icon: 'fa-solid:tachometer-alt' },
  { path: '/client/list', name: 'ClientList', title: '客户端管理', icon: 'fa-solid:desktop' },
  { path: '/listener/list', name: 'ListenerList', title: '监听管理', icon: 'fa-solid:headphones' },
  { path: '/download/index', name: 'DownloadIndex', title: '客户端生成', icon: 'fa-solid:download' },
  { path: '/pluginrunner/index', name: 'PluginRunner', title: '插件运行', icon: 'grommet-icons:action' },
  { path: '/tunnel/list', name: 'TunnelList', title: '隧道代理', icon: 'fa-solid:globe' },
  { path: '/setting/index', name: 'SettingIndex', title: '系统设置', icon: 'fa6-solid:gear' },
  { path: '/about/index', name: 'AboutPage', title: '关于', icon: '' },
]

const routes = [
  { path: '/', redirect: '/dashboard/index' },
  { path: '/login', name: 'Login', component: () => import('../views/LoginView.vue'), meta: { public: true } },
  // 4.9.3 的 SSO 登录入口（这里直接当成登录页；token 由后端校验的版本未实现）
  { path: '/loginSSO/:token', name: 'LoginSSO', component: () => import('../views/LoginView.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('../layout/BasicLayout.vue'),
    children: [
      // ---- 10 仪表盘 ----
      { path: 'dashboard/index', name: 'DashboardIndex', component: () => import('../views/DashboardView.vue') },

      // ---- 11 客户端（/client/list 是列表页）----
      { path: 'client/list', name: 'ClientList', component: () => import('../views/ClientView.vue') },
      { path: 'client/index', redirect: '/client/list' },

      // ---- 22 主机管理（4.9.3 的 /clientManager/index、/clientManager/:id）----
      { path: 'clientManager/index', name: 'ClientManagerIndex', component: () => import('../views/ClientView.vue') },
      { path: 'clientManager/:id', name: 'ClientManagerTab', component: () => import('../views/FilesView.vue') },

      // ---- 12 监听（/listener/list）----
      { path: 'listener/list', name: 'ListenerIndex', component: () => import('../views/ListenerView.vue') },
      { path: 'listener/:id', name: 'ListenerTab', component: () => import('../views/ListenerView.vue') },
      { path: 'listener/index', redirect: '/listener/list' },

      // ---- 13 客户端生成（5 个子路由：同一页面用页签区分）----
      { path: 'download/index', name: 'DownloadIndex', component: () => import('../views/DownloadView.vue') },
      { path: 'download/stage', name: 'DownloadStage', component: () => import('../views/DownloadView.vue') },
      { path: 'download/stageless', name: 'DownloadStageless', component: () => import('../views/DownloadView.vue') },
      { path: 'download/listen', name: 'DownloadListen', component: () => import('../views/DownloadView.vue') },
      { path: 'download/ebpf', name: 'DownloadEbpf', component: () => import('../views/DownloadView.vue') },

      // ---- 14 命令控制 ----
      { path: 'terminal/index', name: 'TerminalIndex', component: () => import('../views/ClientPickerView.vue'), props: { kind: 'terminal' } },
      { path: 'terminal/:id', name: 'TerminalTab', component: () => import('../views/TerminalView.vue') },

      // ---- 15 文件管理 ----
      { path: 'filemanager/index', name: 'FileManagerIndex', component: () => import('../views/ClientPickerView.vue'), props: { kind: 'filemanager' } },
      { path: 'filemanager/:id', name: 'FileManagerTab', component: () => import('../views/FilesView.vue') },

      // ---- 18 插件运行 ----
      { path: 'pluginrunner/index', name: 'PluginRunner', component: () => import('../views/PluginView.vue') },
      { path: 'pluginrunner/:id', name: 'PluginRunnerTab', component: () => import('../views/PluginView.vue') },

      // ---- 19 隧道代理 ----
      { path: 'tunnel/list', name: 'TunnelIndex', component: () => import('../views/TunnelView.vue') },
      { path: 'tunnel/:id', name: 'TunnelTab', component: () => import('../views/TunnelView.vue') },
      { path: 'tunnel/index', redirect: '/tunnel/list' },

      // ---- 20 系统设置 ----
      { path: 'setting/index', name: 'SettingIndex', component: () => import('../views/SettingView.vue') },

      // ---- 23 关于 ----
      { path: 'about/index', name: 'AboutPage', component: () => import('../views/AboutView.vue') },

      // ---- 旧路径兼容（历史链接 / 文档里出现过的 URL）----
      { path: 'clientManager/terminal/:id', redirect: (to: any) => `/terminal/${to.params.id}` },
      { path: 'clientManager/files/:id', redirect: (to: any) => `/filemanager/${to.params.id}` },
      { path: 'client/:id', redirect: '/client/list' },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard/index' },
]

const router = createRouter({ history: createWebHashHistory(), routes })

router.beforeEach((to) => {
  if (to.meta.public) return true
  if (!localStorage.getItem(TOKEN_KEY)) return { path: '/login', query: { redirect: to.fullPath } }
  return true
})

export default router
