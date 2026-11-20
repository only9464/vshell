import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

// 路由与 docs/FRONTEND.md 的页面清单一致；hash 模式避免服务端回退配置
// （原版面板对未知路径返回 404，见 docs/REFERENCE.md R-14）。
const routes = [
  { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('../layouts/BasicLayout.vue'),
    redirect: '/dashboard',
    children: [
      { path: 'dashboard', name: 'dashboard', component: () => import('../views/DashboardView.vue'), meta: { title: '仪表盘' } },
      { path: 'clients', name: 'clients', component: () => import('../views/ClientListView.vue'), meta: { title: '客户端' } },
      { path: 'listeners', name: 'listeners', component: () => import('../views/ListenerListView.vue'), meta: { title: '监听器' } },
      { path: 'download', name: 'download', component: () => import('../views/DownloadView.vue'), meta: { title: '载荷下载' } },
      { path: 'terminal/:id', name: 'terminal', component: () => import('../views/TerminalView.vue'), meta: { title: '终端' } },
      { path: 'files/:id', name: 'files', component: () => import('../views/FilesView.vue'), meta: { title: '文件管理' } },
      { path: 'screen/:id', name: 'screen', component: () => import('../views/ScreenView.vue'), meta: { title: '屏幕直播' } },
      { path: 'tunnels', name: 'tunnels', component: () => import('../views/TunnelView.vue'), meta: { title: '隧道' } },
      { path: 'about', name: 'about', component: () => import('../views/AboutView.vue'), meta: { title: '关于' } },
      { path: 'plugins/:id', name: 'plugins', component: () => import('../views/PluginView.vue'), meta: { title: '插件' } },
      { path: 'settings', name: 'settings', component: () => import('../views/SettingView.vue'), meta: { title: '设置' } },
    ],
  },
]

const router = createRouter({ history: createWebHashHistory(), routes })

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (!to.meta.public && !auth.loggedIn) return { path: '/login' }
  if (to.path === '/login' && auth.loggedIn) return { path: '/dashboard' }
  return true
})

export default router
