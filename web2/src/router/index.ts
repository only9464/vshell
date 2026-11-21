import { createRouter, createWebHashHistory } from 'vue-router'
import { TOKEN_KEY } from '../api/http'

// 路由表按 4.9.3 的菜单结构组织（vben 风格：/dashboard/index、/clientManager/index…）。
// 4.9.3 的 getMenuList 返回 404，说明路由是前端静态定义；这里同样静态定义。
export const menus = [
  { path: '/dashboard/index', name: 'Dashboard', title: '工作台', icon: 'DashboardOutlined' },
  { path: '/clientManager/index', name: 'ClientManager', title: '客户端', icon: 'DesktopOutlined' },
  { path: '/listener/index', name: 'Listener', title: '监听器', icon: 'ApiOutlined' },
  { path: '/download/index', name: 'Download', title: '载荷下载', icon: 'DownloadOutlined' },
  { path: '/tunnel/index', name: 'Tunnel', title: '隧道', icon: 'SwapOutlined' },
  { path: '/pluginrunner/index', name: 'PluginRunner', title: '插件', icon: 'AppstoreOutlined' },
  { path: '/setting/index', name: 'Setting', title: '设置', icon: 'SettingOutlined' },
  { path: '/about/index', name: 'About', title: '关于', icon: 'InfoCircleOutlined' },
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
