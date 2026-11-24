<script setup lang="ts">
// 布局按 4.9.3 的真实界面重做（通过 scripts/serve-493.py 跑起来截图确认）：
//   顶部 header：logo + 管理平台 | 横排菜单 | 明暗/语言/admin/设置/全屏
//   第二行：可关闭的页签（右侧 刷新 / 更多）
//   内容区
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CloseOutlined, ReloadOutlined, SettingOutlined, ExpandOutlined, TranslationOutlined } from '@ant-design/icons-vue'
import { menus } from '../router'
import { useAuthStore } from '../stores/auth'
import { isDark, toggleDark } from '../stores/theme'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

// ---- 页签：按访问顺序记录，可关闭（4.9.3 的 header 下方那一条）----
type Tab = { path: string; title: string }
const tabs = ref<Tab[]>(JSON.parse(sessionStorage.getItem('vshell.tabs') || '[]') as Tab[])
const homeTab: Tab = { path: '/dashboard/index', title: '仪表盘' }
if (!tabs.value.length) tabs.value = [homeTab]

function titleOf(path: string) {
  if (path.startsWith('/clientManager/files') || path.startsWith('/filemanager/')) return '文件管理'
  if (path.startsWith('/clientManager/terminal') || path.startsWith('/terminal/')) return '命令控制'
  if (path.startsWith('/tunnel/')) return '隧道代理'
  const hit = menus.find((m) => path.startsWith(m.path.replace('/index', '').replace('/list', '')))
  return hit ? hit.title : path
}

watch(() => route.path, (p) => {
  if (p === '/' || p.startsWith('/login')) return
  if (!tabs.value.some((t) => t.path === p)) {
    tabs.value.push({ path: p, title: titleOf(p) })
    sessionStorage.setItem('vshell.tabs', JSON.stringify(tabs.value))
  }
}, { immediate: true })

function closeTab(path: string) {
  const idx = tabs.value.findIndex((t) => t.path === path)
  if (idx < 0) return
  tabs.value.splice(idx, 1)
  if (!tabs.value.length) tabs.value = [homeTab]
  sessionStorage.setItem('vshell.tabs', JSON.stringify(tabs.value))
  if (route.path === path) router.push((tabs.value[idx] || tabs.value[idx - 1] || homeTab).path)
}

// 当前高亮：子页面让父菜单高亮
const activeKey = computed(() => {
  const p = route.path
  if (p.startsWith('/clientManager')) return '/client/list'
  if (p.startsWith('/filemanager')) return '/filemanager/index'
  if (p.startsWith('/terminal')) return '/terminal/index'
  if (p.startsWith('/tunnel') || p.startsWith('/tunnel/list')) return '/tunnel/list'
  return p
})

function toggleFullscreen() {
  if (document.fullscreenElement) document.exitFullscreen()
  else document.documentElement.requestFullscreen()
}

function logout() {
  auth.logout?.()
  sessionStorage.removeItem('vshell.tabs')
  router.replace('/login')
}
</script>

<template>
  <div class="shell">
    <!-- header：logo + 标题 | 横排菜单 | 右侧图标组 -->
    <header class="header">
      <div class="brand">
        <span class="logo">VS</span>
        <span class="brand-title">管理平台</span>
      </div>
      <nav class="nav">
        <a v-for="m in menus" :key="m.path" class="nav-item" :class="{ active: activeKey === m.path }"
          @click="router.push(m.path)">
          <span v-if="m.icon" class="nav-ico" :class="`i-${m.icon}`" />
          <span>{{ m.title }}</span>
        </a>
      </nav>
      <div class="actions">
        <a-tooltip :title="isDark ? '亮色主题' : '黑暗主题'">
          <a class="icon-btn" @click="toggleDark"><span class="act i-ant-design:bulb-outlined" /></a>
        </a-tooltip>
        <a-tooltip title="语言"><a class="icon-btn"><TranslationOutlined /></a></a-tooltip>
        <span class="user">{{ auth.username || 'admin' }}</span>
        <a-tooltip title="设置"><a class="icon-btn" @click="router.push('/setting/index')"><SettingOutlined /></a></a-tooltip>
        <a-tooltip title="全屏"><a class="icon-btn" @click="toggleFullscreen"><ExpandOutlined /></a></a-tooltip>
        <a-tooltip title="退出"><a class="icon-btn" @click="logout"><CloseOutlined /></a></a-tooltip>
      </div>
    </header>

    <!-- 页签行 -->
    <div class="tabs">
      <div class="tab-list">
        <span v-for="t in tabs" :key="t.path" class="tab" :class="{ active: route.path === t.path }"
          @click="router.push(t.path)">
          <span class="tab-ico" :class="`i-${(menus.find(m => t.path.startsWith(m.path.replace('/index',''))) || {}).icon || 'ant-design:file-outlined'}`" />
          {{ t.title }}
          <CloseOutlined class="tab-close" @click.stop="closeTab(t.path)" />
        </span>
      </div>
      <div class="tab-tools">
        <a class="icon-btn" @click="router.go(0)"><ReloadOutlined /></a>
        <a class="icon-btn"><span class="act i-ant-design:ellipsis-outlined" /></a>
      </div>
    </div>

    <main class="content">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.shell { display: flex; flex-direction: column; height: 100vh; background: var(--ant-color-bg-layout, #f0f2f5); }
/* header */
.header { display: flex; align-items: center; height: 48px; padding: 0 12px; background: var(--ant-color-bg-container, #fff); border-bottom: 1px solid var(--ant-color-border-split, #f0f0f0); }
.brand { display: flex; align-items: center; gap: 8px; width: 168px; }
.logo { width: 28px; height: 28px; border-radius: 6px; background: #1677ff; color: #fff; font-weight: 700; font-size: 13px; display: flex; align-items: center; justify-content: center; }
.brand-title { font-size: 15px; font-weight: 600; }
.nav { display: flex; align-items: center; gap: 4px; flex: 1; overflow-x: auto; }
.nav-item { display: inline-flex; align-items: center; gap: 6px; height: 48px; padding: 0 12px; font-size: 14px; color: var(--ant-color-text, rgba(0,0,0,.85)); cursor: pointer; white-space: nowrap; border-bottom: 2px solid transparent; }
.nav-item:hover { color: #1677ff; }
.nav-item.active { color: #1677ff; border-bottom-color: #1677ff; }
.nav-ico { width: 16px; height: 16px; display: inline-block; }
.actions { display: flex; align-items: center; gap: 6px; }
.icon-btn { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; border-radius: 4px; cursor: pointer; color: var(--ant-color-text-secondary, rgba(0,0,0,.65)); }
.icon-btn:hover { background: rgba(0,0,0,.04); color: #1677ff; }
.act { width: 16px; height: 16px; display: inline-block; }
.user { font-size: 13px; padding: 0 4px; color: var(--ant-color-text, rgba(0,0,0,.85)); }
/* tabs */
.tabs { display: flex; align-items: center; height: 36px; padding: 0 8px; background: var(--ant-color-bg-container, #fff); border-bottom: 1px solid var(--ant-color-border-split, #f0f0f0); }
.tab-list { display: flex; align-items: center; gap: 6px; flex: 1; overflow-x: auto; }
.tab { display: inline-flex; align-items: center; gap: 6px; height: 24px; padding: 0 8px; font-size: 12px; color: #1677ff; background: rgba(22,119,255,.08); border-radius: 3px; cursor: pointer; white-space: nowrap; }
.tab.active { background: #1677ff; color: #fff; }
.tab-ico { width: 12px; height: 12px; display: inline-block; }
.tab-close { font-size: 10px; opacity: .7; }
.tab-tools { display: flex; align-items: center; gap: 4px; }
/* content */
.content { flex: 1; overflow: auto; padding: 12px; }
</style>
