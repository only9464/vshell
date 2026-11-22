<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
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

// 4.9.3 的流量展示：自动换算 K/M/G，后缀 "/s"
function fmtFlow(v: any) {
  const n = Number(v) || 0
  const unit = ['', 'K', 'M', 'G', 'T']
  let i = 0, x = n
  while (x >= 1024 && i < unit.length - 1) { x /= 1024; i++ }
  return `${x.toFixed(i === 0 ? 0 : 1)}${unit[i]}B/s`
}
const sysInfo = computed(() => [
  { key: 'TCP 通讯 (已建立)', value: info.value.tcpCount ?? 0 },
  { key: 'UDP 通讯 (已建立)', value: info.value.udpCount ?? 0 },
  { key: '流出带宽', value: fmtFlow(info.value.io_send) },
  { key: '流入带宽', value: fmtFlow(info.value.io_recv) },
])
const cfgInfo = computed(() => [
  { key: '日志级别', value: info.value.logLevel ?? '-' },
  { key: '日志文件', value: info.value.logPath || '-' },
  { key: '授权到期时间', value: info.value.licTime || '-' },
  { key: '高级授权', value: info.value.vip ? '是' : '否' },
  { key: '授权客户端数量', value: info.value.clientNum || '-' },
  { key: 'WEB端口', value: info.value.web_port ?? '-' },
  { key: 'Basic认证', value: info.value.web_basic_auth ? '开' : '关' },
  { key: '版本', value: info.value.version || '-' },
])

let timer: number | undefined
onMounted(() => { load(); timer = window.setInterval(load, 1000) })
onBeforeUnmount(() => timer && window.clearInterval(timer))
</script>

<template>
  <a-spin :spinning="loading">
    <!-- KPI：顺序与文案取自 4.9.3（监听总数 / 在线监听数 / 客户端总数 / 在线客户端） -->
    <a-row :gutter="[16, 16]">
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card size="small"><div class="kpi">
          <span class="ico i-flat-ui:magic" />
          <div><div class="kpi-label">监听总数</div><div class="kpi-value">{{ info.listenerCount || 0 }}</div></div>
        </div></a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card size="small"><div class="kpi">
          <span class="ico i-flat-color-icons:headset" />
          <div><div class="kpi-label">在线监听数</div><div class="kpi-value">{{ info.listenerOnlineCount || 0 }}</div></div>
        </div></a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card size="small"><div class="kpi">
          <span class="ico i-flat-ui:responsive" />
          <div><div class="kpi-label">客户端总数</div><div class="kpi-value">{{ info.clientCount || 0 }}</div></div>
        </div></a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :lg="6">
        <a-card size="small"><div class="kpi">
          <span class="ico i-flat-ui:pc" />
          <div><div class="kpi-label">在线客户端</div><div class="kpi-value">{{ info.clientOnlineCount || 0 }}</div></div>
        </div></a-card>
      </a-col>
    </a-row>

    <!-- 系统信息 / 配置信息：字段与 4.9.3 的 S / a 两张表一致 -->
    <a-row :gutter="[16, 16]" style="margin-top: 16px">
      <a-col :xs="24" :lg="12">
        <a-card size="small" title="系统信息">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item v-for="it in sysInfo" :key="it.key" :label="it.key">{{ it.value }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="12">
        <a-card size="small" title="配置信息">
          <a-descriptions :column="1" size="small">
            <a-descriptions-item v-for="it in cfgInfo" :key="it.key" :label="it.key">{{ it.value }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="[16, 16]" style="margin-top: 16px">
      <a-col :xs="24" :lg="12">
        <a-card size="small" title="负载趋势（每 1 秒采样）">
          <div class="chart-row">
            <div><div class="label">CPU</div><Sparkline :values="cpuSeries" /></div>
            <div><div class="label">磁盘</div><Sparkline :values="diskSeries" color="#52c41a" /></div>
          </div>
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
.kpi-label { font-size: 13px; color: rgba(0, 0, 0, 0.45); }
.kpi-value { font-size: 24px; font-weight: 600; line-height: 1.2; }
.ico { width: 34px; height: 34px; flex: 0 0 34px; }
.chart-row { display: flex; gap: 32px; flex-wrap: wrap; }
.label { font-size: 12px; color: rgba(0, 0, 0, 0.45); margin-bottom: 4px; }
</style>
