<script setup lang="ts">
// 交互终端：xterm + /api/terminal/ws（服务端下发原始 shell 字节，按键直接回传）
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { TOKEN_KEY } from '../api/http'

const route = useRoute()
const box = ref<HTMLDivElement>()
const status = ref<'未连接' | '已连接' | '已断开' | '连接出错'>('未连接')
const fontSize = ref(Number(localStorage.getItem('vshell.termFont')) || 14)
let term: Terminal | null = null
let fit: FitAddon | null = null
let ws: WebSocket | null = null

function applyFont(size: number) {
  fontSize.value = Math.min(20, Math.max(12, size))
  if (term) { term.options.fontSize = fontSize.value; fit?.fit() }
  localStorage.setItem('vshell.termFont', String(fontSize.value))
}

function connect() {
  const id = route.params.id
  const token = localStorage.getItem(TOKEN_KEY) || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/terminal/ws?id=${id}&token=${encodeURIComponent(token)}`)
  ws.onopen = () => {
    status.value = '已连接'
    ws?.send(JSON.stringify({ type: '1', cols: term!.cols, rows: term!.rows }))
  }
  ws.onmessage = (ev) => {
    if (typeof ev.data === 'string') term!.write(ev.data)
    else term!.write(new Uint8Array(ev.data))
  }
  ws.onclose = () => { status.value = '已断开' }
  ws.onerror = () => { status.value = '连接出错' }
}

onMounted(() => {
  term = new Terminal({ fontSize: fontSize.value, cursorBlink: true, scrollback: 5000 })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(box.value!)
  fit.fit()
  connect()
  term.onData((d) => ws?.readyState === WebSocket.OPEN && ws.send(d))
  term.onResize(({ cols, rows }) => {
    if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: '1', cols, rows }))
  })
  window.addEventListener('resize', onResize)
})

function onResize() { fit?.fit() }

function clearScreen() {
  term?.clear()
  if (ws?.readyState === WebSocket.OPEN) ws.send(String.fromCharCode(12))
}

async function copySelection() {
  const sel = term?.getSelection() || ''
  if (!sel) { message.warning('先选中要复制的内容'); return }
  try { await navigator.clipboard.writeText(sel); message.success('已复制') }
  catch { message.warning('复制失败，请手动选择') }
}

function reconnect() { location.reload() }

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  ws?.close()
  term?.dispose()
})
</script>

<template>
  <a-card :bordered="false" :body-style="{ padding: '12px' }">
    <div class="toolbar">
      <a-tag :color="status === '已连接' ? 'green' : 'default'">{{ status }}</a-tag>
      <span class="mono">客户端 #{{ route.params.id }}</span>
      <a-space size="small">
        <a-button size="small" @click="applyFont(fontSize - 1)">A-</a-button>
        <a-button size="small" @click="applyFont(fontSize + 1)">A+</a-button>
      </a-space>
      <a-button size="small" @click="clearScreen">清屏</a-button>
      <a-button size="small" @click="copySelection">复制选中</a-button>
      <a-button size="small" :disabled="status === '已连接'" @click="reconnect">重连</a-button>
      <span class="hint mono">字号 {{ fontSize }}px</span>
    </div>
    <div ref="box" class="term-box" />
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 8px; }
.term-box { height: calc(100vh - 240px); background: #0b1020; border-radius: 6px; padding: 8px; }
.hint { color: rgba(0, 0, 0, 0.45); font-size: 12px; }
</style>
