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
const kpis = computed(() => [
  { title: '监听总数', value: info.value.listenerCount || 0, icon: 'flat-ui:magic' },
  { title: '在线监听数', value: info.value.listenerOnlineCount || 0, icon: 'flat-color-icons:headset' },
  { title: '客户端总数', value: info.value.clientCount || 0, icon: 'flat-ui:responsive' },
  { title: '在线客户端', value: info.value.clientOnlineCount || 0, icon: 'flat-ui:pc' },
])
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
    <!-- KPI：4.9.3 的 PortCard —— 卡片标题是统计项，卡片内「大数字在左、40px 图标在右」 -->
    <div class="ports">
      <a-card v-for="k in kpis" :key="k.title" size="small" :title="k.title" class="port-card">
        <div class="port-body">
          <h1 class="port-num">{{ k.value }}</h1>
          <span class="port-ico" :class="`i-${k.icon}`" />
        </div>
      </a-card>
    </div>

    <!-- 系统信息 / 配置信息：4.9.3 的 SystemCard + ConfigCard，行内 key 左、value 右 -->
    <a-row :gutter="[16, 16]" style="margin-top: 16px">
      <a-col :xs="24" :lg="12">
        <a-card size="small" title="配置信息">
          <div v-for="it in cfgInfo" :key="it.key" class="kv-row">
            <span>{{ it.key }}</span><span class="mono">{{ it.value }}</span>
          </div>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="12">
        <a-card size="small" title="系统信息">
          <div class="meter"><span>处理器</span><a-progress :percent="Math.round(Number(info.cpu) || 0)" size="small" status="active" /></div>
          <div class="meter"><span>磁盘</span><a-progress :percent="Math.round(Number(info.disk) || 0)" size="small" status="active" /></div>
          <div class="meter"><span>内存</span><a-progress :percent="Math.round(Number(info.virtual_mem) || 0)" size="small" status="active" /></div>
          <div class="meter"><span>虚拟内存</span><a-progress :percent="Math.round(Number(info.swap_mem) || 0)" size="small" status="active" /></div>
          <div v-for="it in sysInfo" :key="it.key" class="kv-row">
            <span>{{ it.key }}</span><span class="mono">{{ it.value }}</span>
          </div>
        </a-card>
      </a-col>
    </a-row>

  </a-spin>
</template>

<style scoped>
.sub { font-size: 12px; color: rgba(0, 0, 0, 0.45); }
.kpi { display: flex; align-items: center; gap: 12px; }
.ports { display: flex; flex-wrap: wrap; gap: 16px; }
.port-card { flex: 1 1 220px; }
.port-body { display: flex; align-items: center; justify-content: space-between; padding: 4px 2px; }
.port-num { font-size: 30px; font-weight: 600; margin: 0; line-height: 1.1; }
.port-ico { width: 40px; height: 40px; display: inline-block; background-size: contain; background-repeat: no-repeat; }
.kv-row { display: flex; justify-content: space-between; padding: 6px 2px; font-size: 13px; border-bottom: 1px dashed rgba(0,0,0,.06); }
.kv-row:last-child { border-bottom: none; }
.meter { display: flex; align-items: center; gap: 12px; margin-bottom: 6px; }
.meter > span:first-child { width: 64px; font-size: 13px; color: rgba(0, 0, 0, 0.45); }
.meter :deep(.ant-progress) { flex: 1; margin: 0; }
.kpi-label { font-size: 13px; color: rgba(0, 0, 0, 0.45); }
.kpi-value { font-size: 24px; font-weight: 600; line-height: 1.2; }
.ico { width: 34px; height: 34px; flex: 0 0 34px; }
.chart-row { display: flex; gap: 32px; flex-wrap: wrap; }
.label { font-size: 12px; color: rgba(0, 0, 0, 0.45); margin-bottom: 4px; }
</style>
