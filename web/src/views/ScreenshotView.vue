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
    <el-alert v-if="error" type="warning" :closable="false" :title="error" style="margin-bottom: 12px" />
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
</style>
