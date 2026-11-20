<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { dashboardApi } from '../api'

const info = ref<Record<string, any>>({})
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    info.value = await dashboardApi.info()
  } finally {
    loading.value = false
  }
}
onMounted(load)
</script>

<template>
  <div class="page" v-loading="loading">
    <el-row :gutter="12">
      <el-col :span="6"><el-card><div class="kpi"><div class="label">客户端</div>
        <div class="value">{{ info.clientCount ?? 0 }}</div>
        <div class="sub">在线 {{ info.clientOnlineCount ?? 0 }}</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="kpi"><div class="label">监听器</div>
        <div class="value">{{ info.listenerCount ?? 0 }}</div>
        <div class="sub">运行 {{ info.listenerOnlineCount ?? 0 }}</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="kpi"><div class="label">CPU / 磁盘</div>
        <div class="value">{{ info.cpu ?? 0 }}% / {{ info.disk ?? 0 }}%</div>
        <div class="sub mono">{{ info.load }}</div></div></el-card></el-col>
      <el-col :span="6"><el-card><div class="kpi"><div class="label">版本</div>
        <div class="value">{{ info.version }}</div>
        <div class="sub">许可 {{ info.licTime }}</div></div></el-card></el-col>
    </el-row>
    <el-card style="margin-top: 12px">
      <template #header>运行信息</template>
      <el-descriptions :column="3" border size="small">
        <el-descriptions-item label="TCP/UDP">{{ info.tcp }}/{{ info.udp }}</el-descriptions-item>
        <el-descriptions-item label="IO 收/发">{{ info.io_recv }}/{{ info.io_send }}</el-descriptions-item>
        <el-descriptions-item label="Web 端口">{{ info.web_port }}</el-descriptions-item>
        <el-descriptions-item label="Basic Auth">{{ info.web_basic_auth ? '开' : '关' }}</el-descriptions-item>
        <el-descriptions-item label="日志级别">{{ info.logLevel }}</el-descriptions-item>
        <el-descriptions-item label="客户端上限">{{ info.clientNum }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<style scoped>
.kpi .label { color: #6b7280; font-size: 13px; }
.kpi .value { font-size: 24px; font-weight: 600; margin: 4px 0; }
.kpi .sub { color: #9ca3af; font-size: 12px; }
</style>
