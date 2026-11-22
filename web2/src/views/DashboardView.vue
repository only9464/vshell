<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { get, post } from '../api/http'
import Sparkline from '../components/Sparkline.vue'

const info = ref<Record<string, any>>({})
const recent = ref<any[]>([])
const loading = ref(false)
const cpuSeries = ref<number[]>([])
const diskSeries = ref<number[]>([])
const MAX = 30

const recentColumns = [
  { title: '主机名', dataIndex: 'HostName' },
  { title: '用户', dataIndex: 'UserName', width: 110 },
  { title: '系统', dataIndex: 'OsName', width: 100 },
  { title: '内网 IP', dataIndex: 'LocalIP', width: 140 },
  { title: '在线', dataIndex: 'IsConnect', width: 90 },
]

async function load() {
  loading.value = true
  try {
    // /dashboard/info 是 GET（4.9.3 里少数 GET 接口之一），用 POST 会 404
    const res: any = await get('/dashboard/info')
    info.value = res?.result || {}
    cpuSeries.value = [...cpuSeries.value, Number(info.value.cpu) || 0].slice(-MAX)
    diskSeries.value = [...diskSeries.value, Number(info.value.disk) || 0].slice(-MAX)
    try {
      const c: any = await post('/client/list', { page: 1, pageSize: 50 })
      recent.value = [...(c?.result?.items || [])]
        .sort((a, b) => Number(b.PingCheckTime || 0) - Number(a.PingCheckTime || 0))
        .slice(0, 5)
    } catch { /* 概览不因客户端接口失败而整体报错 */ }
  } finally {
    loading.value = false
  }
}

let timer: number | undefined
onMounted(() => { load(); timer = window.setInterval(load, 4000) })
onBeforeUnmount(() => timer && window.clearInterval(timer))
</script>

<template>
  <a-spin :spinning="loading">
    <a-row :gutter="[16, 16]">
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card size="small">
          <div class="kpi">
            <span class="ico i-flat-ui:pc" />
            <a-statistic title="客户端总数" :value="info.clientCount || 0" />
          </div>
          <div class="sub">在线客户端 {{ info.clientOnlineCount || 0 }}</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card size="small">
          <div class="kpi">
            <span class="ico i-flat-color-icons:headset" />
            <a-statistic title="监听总数" :value="info.listenerCount || 0" />
          </div>
          <div class="sub">在线监听数 {{ info.listenerOnlineCount || 0 }}</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card size="small">
          <div class="kpi">
            <span class="ico i-flat-ui:responsive" />
            <a-statistic title="CPU" :value="info.cpu || 0" suffix="%" />
          </div>
          <div class="sub">磁盘 {{ info.disk || 0 }}%</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card size="small">
          <div class="kpi">
            <span class="ico i-flat-ui:magic" />
            <a-statistic title="版本" :value="info.version || '-'" />
          </div>
          <div class="sub">复刻研究版</div>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="[16, 16]" style="margin-top: 16px">
      <a-col :xs="24" :lg="12">
        <a-card size="small" title="负载趋势（每 4 秒采样）">
          <div class="chart-row">
            <div><div class="label">CPU</div><Sparkline :values="cpuSeries" /></div>
            <div><div class="label">磁盘</div><Sparkline :values="diskSeries" color="#52c41a" /></div>
          </div>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="12">
        <a-card size="small" title="运行信息">
          <a-descriptions :column="2" size="small">
            <a-descriptions-item label="TCP/UDP">{{ info.tcp }}/{{ info.udp }}</a-descriptions-item>
            <a-descriptions-item label="Web 端口">{{ info.web_port }}</a-descriptions-item>
            <a-descriptions-item label="日志级别">{{ info.logLevel }}</a-descriptions-item>

            <a-descriptions-item label="IO 收/发">{{ info.io_recv }}/{{ info.io_send }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>

    <a-card size="small" title="最近上线客户端" style="margin-top: 16px">
      <a-table :columns="recentColumns" :data-source="recent" row-key="Id" size="small" :pagination="false">
        <template #bodyCell="{ column, record }">
          <template v-if="column.dataIndex === 'IsConnect'">
            <a-tag :color="record.IsConnect ? 'green' : 'default'">{{ record.IsConnect ? '在线' : '离线' }}</a-tag>
          </template>
        </template>
      </a-table>
    </a-card>
  </a-spin>
</template>

<style scoped>
.sub { font-size: 12px; color: rgba(0, 0, 0, 0.45); }
.kpi { display: flex; align-items: center; gap: 12px; }
.ico { width: 34px; height: 34px; flex: 0 0 34px; }
.chart-row { display: flex; gap: 32px; flex-wrap: wrap; }
.label { font-size: 12px; color: rgba(0, 0, 0, 0.45); margin-bottom: 4px; }
</style>
