<script setup lang="ts">
// 布局对齐 4.9.3（vben-admin 风格）：
// 左侧深色 Sider（可折叠）+ 顶部 Header（折叠按钮 / 面包屑 / 用户菜单）+ 页签栏 + 内容区。
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ApiOutlined, AppstoreOutlined, DashboardOutlined, DesktopOutlined, DownloadOutlined,
  InfoCircleOutlined, MenuFoldOutlined, MenuUnfoldOutlined, SettingOutlined, SwapOutlined,
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

const current = computed(() => menus.find((m) => route.path.startsWith(m.path.split('/').slice(0, 2).join('/'))) || menus[0])
const selectedKeys = computed(() => [String(route.path)])

function go(path: string) { router.push(path) }

const dark = ref(document.documentElement.getAttribute('data-theme') === 'dark')
function toggleTheme() {
  dark.value = !dark.value
  document.documentElement.setAttribute('data-theme', dark.value ? 'dark' : 'light')
  localStorage.setItem('vshell.theme', dark.value ? 'dark' : 'light')
}

function onUserMenu({ key }: { key: string }) {
  if (key === 'logout') {
    auth.logout()
    router.push('/login')
  }
}
</script>

<template>
  <a-layout style="height: 100%">
    <a-layout-sider
      :collapsed="collapsed" :width="210" :collapsed-width="64" collapsible
      :trigger="null" theme="dark" class="sider"
    >
      <div class="logo">
        <span class="logo-mark">VS</span>
        <span v-show="!collapsed" class="logo-text">管理平台</span>
      </div>
      <a-menu v-model:selectedKeys="selectedKeys" theme="dark" mode="inline" @click="(e: any) => go(String(e.key))">
        <a-menu-item v-for="m in menus" :key="m.path">
          <component :is="iconMap[m.icon]" />
          <span>{{ m.title }}</span>
        </a-menu-item>
      </a-menu>
    </a-layout-sider>

    <a-layout>
      <a-layout-header class="header">
        <span data-test="nav-toggle" class="trigger" @click="toggleNav">
          <component :is="collapsed ? MenuUnfoldOutlined : MenuFoldOutlined" />
        </span>
        <a-breadcrumb class="crumb">
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
      </a-layout-header>
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
.trigger { font-size: 18px; cursor: pointer; }
.crumb { margin-left: 4px; }
.spacer { flex: 1; }
.user { color: rgba(0, 0, 0, 0.85); }
.content { padding: 16px; overflow: auto; background: #f5f5f5; }
</style>
