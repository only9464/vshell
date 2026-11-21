<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { get } from '../api/http'

// 截图（B6.7 后半）：GET /api/screenshot/get 返回**裸 base64 PNG**，
// 不是 JSON 信封，因此这里直接用 axios 原始响应（responseType: 'text'）。
const route = useRoute()
const id = Number(route.params.id)
const quality = ref<'big' | 'normal' | 'small'>('normal')
const src = ref('')
const error = ref('')
const loading = ref(false)

async function shoot() {
  loading.value = true
  error.value = ''
  try {
    const token = localStorage.getItem('vshell.token') || ''
    const resp = await fetch(`/api/screenshot/get?id=${id}&quality=${quality.value}&token=${encodeURIComponent(token)}`)
    const text = await resp.text()
    if (!text || text.startsWith('{')) {
      error.value = text || `HTTP ${resp.status}`
      src.value = ''
      return
    }
    src.value = `data:image/png;base64,${text.trim()}`
  } catch (e: any) {
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}
onMounted(shoot)
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="toolbar">
      <el-select v-model="quality" style="width: 120px" @change="shoot">
        <el-option label="高清" value="big" />
        <el-option label="普通" value="normal" />
        <el-option label="流畅" value="small" />
      </el-select>
      <el-button type="primary" @click="shoot">重新截图</el-button>
      <span class="mono">客户端 #{{ id }}</span>
    </div>
    <el-alert v-if="error" type="warning" :closable="false" style="margin-bottom: 12px">
      <template #title>截图失败：{{ error }}</template>
      <div class="hint">
        目标机需要可用的截屏工具，按环境安装其一即可：
        <span class="mono">grim</span>（Wayland）、
        <span class="mono">imagemagick</span>（X11，提供 import）、
        <span class="mono">scrot</span>、<span class="mono">gnome-screenshot</span>、
        <span class="mono">spectacle</span>（KDE）。
        无图形会话（纯终端/无合成器）时无法截图，这是环境限制而非程序故障。
      </div>
    </el-alert>
    <div class="stage">
      <img v-if="src" :src="src" class="shot" alt="screenshot" />
      <span v-else class="placeholder">尚无截图</span>
    </div>
  </div>
</template>

<style scoped>
.stage { background: #0b1020; min-height: calc(100vh - 200px); display: flex; align-items: center; justify-content: center; border-radius: 6px; }
.shot { max-width: 100%; }
.placeholder { color: #6b7280; }
.hint { font-size: 12px; line-height: 1.6; margin-top: 4px; }
</style>
