<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { dashboardApi } from '../api'
import Sparkline from '../components/Sparkline.vue'

const info = ref<Record<string, any>>({})
const loading = ref(false)

// 采样序列（保留最近 30 个点，约 1 分钟）
const MAX_POINTS = 30
const cpuSeries = ref<number[]>([])
const memSeries = ref<number[]>([])

async function load() {
  loading.value = true
  try {
    info.value = await dashboardApi.info()
    cpuSeries.value = [...cpuSeries.value, Number(info.value.cpu) || 0].slice(-MAX_POINTS)
    memSeries.value = [...memSeries.value, Number(info.value.disk) || 0].slice(-MAX_POINTS)
  } finally {
    loading.value = false
  }
}

// 每 2 秒采样一次；组件卸载时清掉定时器
let timer: number | undefined
onMounted(() => {
  load()
  timer = window.setInterval(load, 2000)
})
onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
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
      <template #header>负载趋势（每 2 秒采样，保留最近 30 个点）</template>

<style scoped>
.charts { display: flex; gap: 32px; flex-wrap: wrap; }
.chart-label { font-size: 12px; color: var(--el-text-color-secondary); margin-bottom: 4px; }
</style>
      <div class="charts">
        <div class="chart">
          <div class="chart-label">CPU</div>
          <Sparkline :values="cpuSeries" color="#409eff" />
        </div>
        <div class="chart">
          <div class="chart-label">磁盘/内存占用</div>
          <Sparkline :values="memSeries" color="#67c23a" />
        </div>
      </div>
    </el-card>
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
