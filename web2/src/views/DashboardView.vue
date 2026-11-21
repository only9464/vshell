<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { get } from '../api/http'

const info = ref<Record<string, any>>({})
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    info.value = (await get('/dashboard/info'))?.result || {}
  } finally {
    loading.value = false
  }
}
let timer: number | undefined
onMounted(() => { load(); timer = window.setInterval(load, 5000) })
onBeforeUnmount(() => timer && window.clearInterval(timer))
</script>

<template>
  <a-spin :spinning="loading">
    <a-row :gutter="[16, 16]">
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card><a-statistic title="客户端" :value="info.clientCount || 0">
          <template #suffix><span class="sub">在线 {{ info.clientOnlineCount || 0 }}</span></template>
        </a-statistic></a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card><a-statistic title="监听器" :value="info.listenerCount || 0">
          <template #suffix><span class="sub">运行 {{ info.listenerOnlineCount || 0 }}</span></template>
        </a-statistic></a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card><a-statistic title="CPU" :value="info.cpu || 0" suffix="%" /></a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card><a-statistic title="版本" :value="info.version || '-'" /></a-card>
      </a-col>
    </a-row>
  </a-spin>
</template>

<style scoped>
.sub { font-size: 12px; color: rgba(0, 0, 0, 0.45); }
</style>
