<script setup lang="ts">
// 截图：GET /api/screenshot/get 返回裸 base64 PNG
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { TOKEN_KEY } from '../api/http'

const route = useRoute()
const id = Number(route.params.id)
const quality = ref('normal')
const src = ref('')
const error = ref('')
const loading = ref(false)

async function shoot() {
  loading.value = true
  error.value = ''
  try {
    const token = localStorage.getItem(TOKEN_KEY) || ''
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
  } finally { loading.value = false }
}
onMounted(shoot)
</script>

<template>
  <a-card :bordered="false" :body-style="{ padding: '12px' }">
    <div class="toolbar">
      <a-select v-model:value="quality" style="width: 120px" @change="shoot">
        <a-select-option value="big">高清</a-select-option>
        <a-select-option value="normal">普通</a-select-option>
        <a-select-option value="small">流畅</a-select-option>
      </a-select>
      <a-button type="primary" @click="shoot">重新截图</a-button>
      <span class="mono">客户端 #{{ id }}</span>
    </div>

    <a-alert v-if="error" type="warning" show-icon style="margin-bottom: 12px">
      <template #message>截图失败</template>
      <template #description>
        <div class="err">{{ error }}</div>
        <div class="hint">
          目标机需要可用的截屏工具：<span class="mono">grim</span>（Wayland）、
          <span class="mono">imagemagick</span>（X11）、<span class="mono">scrot</span>、
          <span class="mono">gnome-screenshot</span>、<span class="mono">spectacle</span>。
          无图形会话时无法截图，属环境限制。
        </div>
      </template>
    </a-alert>

    <div class="stage">
      <img v-if="src" :src="src" alt="screenshot" />
      <span v-else class="placeholder">尚无截图</span>
    </div>
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; }
.stage { background: #0b1020; min-height: calc(100vh - 260px); display: flex; align-items: center; justify-content: center; border-radius: 6px; }
img { max-width: 100%; }
.placeholder { color: #6b7280; }
.err { font-family: ui-monospace, monospace; font-size: 12px; }
.hint { font-size: 12px; margin-top: 6px; line-height: 1.6; }
</style>
