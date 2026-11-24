import { createRouter, createWebHashHistory } from 'vue-router'
import { TOKEN_KEY } from '../api/http'

// 路由表按 4.9.3 的菜单结构组织（vben 风格：/dashboard/index、/clientManager/index…）。
// 4.9.3 的 getMenuList 返回 404，说明路由是前端静态定义；这里同样静态定义。
// 菜单文案取自 4.9.3 前端 locale（static/assets/vB2dXDAOV.js）：
// 客户端列表/监听列表/命令控制/隧道代理/插件运行/文件管理/系统设置/客户端生成
// 菜单图标也用 4.9.3 bundle 里出现过的 iconify 图标（ant-design / fa6-solid / ion / grommet-icons）
export const menus = [
  { path: '/dashboard/index', name: 'Dashboard', title: '工作台', icon: 'ant-design:home-outlined' },
  { path: '/clientManager/index', name: 'ClientManager', title: '客户端列表', icon: 'fa6-solid:desktop' },
  { path: '/listener/index', name: 'Listener', title: '监听列表', icon: 'fa6-solid:headphones' },
  { path: '/download/index', name: 'Download', title: '客户端生成', icon: 'fa6-solid:download' },
  { path: '/tunnel/index', name: 'Tunnel', title: '隧道代理', icon: 'ant-design:swap-outlined' },
  { path: '/pluginrunner/index', name: 'PluginRunner', title: '插件运行', icon: 'grommet-icons:services' },
  { path: '/setting/index', name: 'Setting', title: '系统设置', icon: 'ion:settings-outline' },
  { path: '/about/index', name: 'About', title: '关于', icon: 'grommet-icons:action' },
]

const routes = [
  { path: '/', redirect: '/dashboard/index' },
  { path: '/login', name: 'Login', component: () => import('../views/LoginView.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('../layout/BasicLayout.vue'),
    children: [
      { path: 'dashboard/index', component: () => import('../views/DashboardView.vue') },
      { path: 'clientManager/index', component: () => import('../views/ClientView.vue') },
      { path: 'clientManager/terminal/:id', component: () => import('../views/TerminalView.vue') },
      { path: 'clientManager/files/:id', component: () => import('../views/FilesView.vue') },
      // 4.9.3 的独立路由：/filemanager、/terminal（我们历史上把它们挂在
      // clientManager 下）。这里加别名，两种 URL 都能进同一个页面。
      { path: 'filemanager/:id', component: () => import('../views/FilesView.vue') },
      { path: 'filemanager', redirect: '/clientManager/index' },
      { path: 'terminal/:id', component: () => import('../views/TerminalView.vue') },
      { path: 'client/:id', redirect: (to: any) => `/clientManager/index` },
      { path: 'listener/index', component: () => import('../views/ListenerView.vue') },
      { path: 'download/index', component: () => import('../views/DownloadView.vue') },
      { path: 'tunnel/index', component: () => import('../views/TunnelView.vue') },
      { path: 'pluginrunner/index', component: () => import('../views/PluginView.vue') },
      { path: 'setting/index', component: () => import('../views/SettingView.vue') },
      { path: 'about/index', component: () => import('../views/AboutView.vue') },
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
