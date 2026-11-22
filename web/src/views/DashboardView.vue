<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { clientApi, dashboardApi, type ClientItem } from '../api'
import Sparkline from '../components/Sparkline.vue'

const info = ref<Record<string, any>>({})
const loading = ref(false)

// 采样序列（保留最近 30 个点，约 1 分钟）
const MAX_POINTS = 30
const recent = ref<ClientItem[]>([])
const cpuSeries = ref<number[]>([])
const memSeries = ref<number[]>([])

async function load() {
  loading.value = true
  try {
    info.value = await dashboardApi.info()
    // 最近上线：按最后活跃时间倒序取前 5 台
    try {
      const res = await clientApi.list(1, 50, '')
      recent.value = [...(res.items || [])]
        .sort((a, b) => Number(b.PingCheckTime || 0) - Number(a.PingCheckTime || 0))
        .slice(0, 5)
    } catch {
      // 概览页不因客户端接口失败而整体报错
    }
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
        <div class="sub">复刻研究版</div></div></el-card></el-col>
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
      <template #header>最近上线客户端</template>
      <el-table :data="recent" size="small" border>
        <el-table-column prop="HostName" label="主机名" min-width="140" />
        <el-table-column prop="UserName" label="用户" width="110" />
        <el-table-column prop="OsName" label="系统" width="110" />
        <el-table-column prop="LocalIP" label="内网 IP" width="130" />
        <el-table-column label="在线" width="80">
          <template #default="{ row }">
            <el-tag :type="row.IsConnect ? 'success' : 'info'" size="small">{{ row.IsConnect ? '在线' : '离线' }}</el-tag>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="还没有客户端上线" />
        </template>
      </el-table>
      <div style="margin-top: 8px">
        <el-button link type="primary" @click="$router.push('/clients')">查看全部客户端</el-button>
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
      </el-descriptions>
    </el-card>
  </div>
</template>

<style scoped>
.kpi .label { color: #6b7280; font-size: 13px; }
.kpi .value { font-size: 24px; font-weight: 600; margin: 4px 0; }
.kpi .sub { color: #9ca3af; font-size: 12px; }
</style>
