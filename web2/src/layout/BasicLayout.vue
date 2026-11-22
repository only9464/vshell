<script setup lang="ts">
// 布局对齐 4.9.3（vben-admin 风格）：
// 深色 Sider（#001529，可折叠）+ Header（折叠/面包屑/主题/用户）+ 页签栏 + 内容区。
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ApiOutlined, AppstoreOutlined, CloseOutlined, DashboardOutlined, DesktopOutlined,
  DownloadOutlined, InfoCircleOutlined, MenuFoldOutlined, MenuUnfoldOutlined,
  SettingOutlined, SwapOutlined,
} from '@ant-design/icons-vue'
import { menus } from '../router'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const collapsed = ref(localStorage.getItem('vshell.nav') === 'collapsed')
function toggleNav() {
  collapsed.value = !collapsed.value
  localStorage.setItem('vshell.nav', collapsed.value ? 'collapsed' : 'expanded')
}

const iconMap: Record<string, any> = {
  DashboardOutlined, DesktopOutlined, ApiOutlined, DownloadOutlined,
  SwapOutlined, AppstoreOutlined, SettingOutlined, InfoCircleOutlined,
}

// ---- 页签栏（vben 的多标签页）：按访问顺序记录，可关闭 ----
type Tab = { path: string; title: string }
const tabs = ref<Tab[]>(
  JSON.parse(sessionStorage.getItem('vshell.tabs') || '[]') as Tab[],
)

const titleOf = (path: string) => {
  const hit = menus.find((m) => path.startsWith(m.path.split('/').slice(0, 2).join('/')))
  if (hit) return hit.title
  if (path.includes('/terminal/')) return '终端'
  if (path.includes('/files/')) return '文件管理'
  if (path.startsWith('/login')) return '登录'
  return path
}

watch(
  () => route.path,
  (p) => {
    if (p === '/' || p.startsWith('/login')) return
    if (!tabs.value.some((t) => t.path === p)) {
      tabs.value.push({ path: p, title: titleOf(p) })
      sessionStorage.setItem('vshell.tabs', JSON.stringify(tabs.value))
    }
  },
  { immediate: true },
)

function closeTab(path: string) {
  const idx = tabs.value.findIndex((t) => t.path === path)
  if (idx < 0) return
  tabs.value.splice(idx, 1)
  sessionStorage.setItem('vshell.tabs', JSON.stringify(tabs.value))
  if (route.path === path) {
    const next = tabs.value[idx] || tabs.value[idx - 1] || { path: '/dashboard/index' }
    router.push(next.path)
  }
}

// 子页面（/clientManager/terminal/:id、/clientManager/files/:id）也要让"客户端"菜单保持高亮
const current = computed(() => menus.find((m) => route.path.startsWith(m.path.split('/').slice(0, 2).join('/'))) || menus[0])
const selectedKeys = computed(() => [String(current.value.path)])
const openKeys = computed(() => [String(current.value.path.split('/')[1])])

const dark = ref(document.documentElement.getAttribute('data-theme') === 'dark')
function toggleTheme() {
  dark.value = !dark.value
  document.documentElement.setAttribute('data-theme', dark.value ? 'dark' : 'light')
  localStorage.setItem('vshell.theme', dark.value ? 'dark' : 'light')
}

function onUserMenu({ key }: { key: string }) {
  if (key === 'logout') {
    auth.logout()
    sessionStorage.removeItem('vshell.tabs')
    router.push('/login')
  }
}
</script>

<template>
  <a-layout style="height: 100%">
    <a-layout-sider
      :collapsed="collapsed" :width="210" :collapsed-width="64" collapsible
      :trigger="null" theme="dark" class="sider" :style="{ background: '#001529' }"
    >
      <div class="logo">
        <span class="logo-mark">VS</span>
        <span v-show="!collapsed" class="logo-text">管理平台</span>
      </div>
      <a-menu
        v-model:selectedKeys="selectedKeys" :open-keys="openKeys" theme="dark" mode="inline"
        :style="{ background: 'transparent' }" @click="(e: any) => router.push(String(e.key))"
      >
        <a-menu-item v-for="m in menus" :key="m.path">
          <component :is="iconMap[m.icon]" />
          <span>{{ m.title }}</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>

    <a-layout>
      <a-header class="header">
        <span data-test="nav-toggle" class="trigger" @click="toggleNav">
          <component :is="collapsed ? MenuUnfoldOutlined : MenuFoldOutlined" />
        </span>
        <a-breadcrumb class="crumb">
          <a-breadcrumb-item>首页</a-breadcrumb-item>
          <a-breadcrumb-item>{{ current.title }}</a-breadcrumb-item>
        </a-breadcrumb>
        <div class="spacer" />
        <a-button type="text" @click="toggleTheme">{{ dark ? '浅色' : '暗色' }}</a-button>
        <a-dropdown>
          <a class="user" @click.prevent>{{ auth.username || 'admin' }}</a>
          <template #overlay>
            <a-menu @click="onUserMenu">
              <a-menu-item key="logout">退出登录</a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </a-header>

      <!-- 页签栏（对齐 vben 的多标签） -->
      <div class="tabs">
        <span
          v-for="t in tabs" :key="t.path" class="tab" :class="{ active: route.path === t.path }"
          @click="router.push(t.path)"
        >
          {{ t.title }}
          <CloseOutlined class="tab-close" @click.stop="closeTab(t.path)" />
        </span>
      </div>

      <a-layout-content class="content">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<style scoped>
.sider { overflow: auto; }
.logo { height: 56px; display: flex; align-items: center; gap: 8px; padding: 0 16px; color: #fff; }
.logo-mark { width: 26px; height: 26px; border-radius: 6px; background: #1677ff; display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 700; }
.logo-text { font-weight: 600; letter-spacing: 1px; white-space: nowrap; }
.header { display: flex; align-items: center; gap: 12px; background: var(--vshell-header-bg, #fff); padding: 0 16px; height: 56px; line-height: 56px; border-bottom: 1px solid #f0f0f0; }
.trigger { font-size: 18px; cursor: pointer; display: inline-flex; align-items: center; }
.crumb { margin-left: 4px; }
.spacer { flex: 1; }
.user { color: rgba(0, 0, 0, 0.85); }
.tabs { display: flex; gap: 6px; padding: 6px 12px 0; background: #fff; border-bottom: 1px solid #f0f0f0; overflow-x: auto; }
.tab { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; font-size: 12px; border: 1px solid #e5e7eb; border-bottom: none; border-radius: 4px 4px 0 0; background: #fafafa; cursor: pointer; white-space: nowrap; }
.tab.active { background: #fff; color: #1677ff; border-color: #1677ff; }
.tab-close { font-size: 10px; opacity: .5; }
.tab-close:hover { opacity: 1; }
.content { padding: 16px; overflow: auto; background: #f5f5f5; }
</style>
