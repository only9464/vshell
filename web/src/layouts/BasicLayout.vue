<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const menus = [
  { path: '/dashboard', label: '仪表盘', icon: 'Odometer' },
  { path: '/clients', label: '客户端', icon: 'Monitor' },
  { path: '/listeners', label: '监听器', icon: 'Connection' },
  { path: '/download', label: '载荷下载', icon: 'Download' },
  { path: '/settings', label: '设置', icon: 'Setting' },
]

const active = computed(() => route.path)
function go(path: string) {
  router.push(path)
}
function logout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <el-container style="height: 100%">
    <el-aside width="200px" class="aside">
      <div class="brand">VShell</div>
      <el-menu :default-active="active" class="menu">
        <el-menu-item v-for="m in menus" :key="m.path" :index="m.path" @click="go(m.path)">
          <span>{{ m.label }}</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <span class="title">{{ route.meta.title || '' }}</span>
        <span class="spacer" />
        <span class="user">{{ auth.username }}</span>
        <el-button link type="primary" @click="logout">退出</el-button>
      </el-header>
      <el-main>
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.aside { background: #1f2937; color: #e5e7eb; }
.brand { height: 56px; display: flex; align-items: center; justify-content: center; font-weight: 700; letter-spacing: 1px; color: #fff; }
.menu { border-right: none; background: transparent; }
:deep(.el-menu-item) { color: #cbd5e1; }
:deep(.el-menu-item.is-active) { color: #fff; background: #374151; }
.header { display: flex; align-items: center; gap: 12px; background: #fff; border-bottom: 1px solid #e5e7eb; }
.title { font-weight: 600; }
.spacer { flex: 1; }
.user { color: #6b7280; }
</style>
