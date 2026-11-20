<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'

// 面板终端：连 /api/terminal/ws?id=<clientID>&token=<jwt>
// 服务端下发的是原始 shell 字节（文本帧），按键直接回传文本帧；
// resize 用 {"type":"1","cols","rows"}（见 docs/API.md §8.1）。
const route = useRoute()
const box = ref<HTMLDivElement>()
const status = ref('未连接')
let term: Terminal | null = null
let fit: FitAddon | null = null
let ws: WebSocket | null = null

onMounted(() => {
  const id = route.params.id
  const token = localStorage.getItem('vshell.token') || ''
  term = new Terminal({ fontSize: 14, cursorBlink: true, scrollback: 5000 })
  fit = new FitAddon()
  term.loadAddon(fit)
  term.open(box.value!)
  fit.fit()

  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const url = `${proto}://${location.host}/api/terminal/ws?id=${id}&token=${encodeURIComponent(token)}`
  ws = new WebSocket(url)
  ws.onopen = () => {
    status.value = '已连接'
    ws?.send(JSON.stringify({ type: '1', cols: term!.cols, rows: term!.rows }))
  }
  ws.onmessage = (ev) => {
    // 服务端可能发文本或二进制帧：字符串直接写，二进制按 UTF-8 解码写。
    if (typeof ev.data === 'string') term!.write(ev.data)
    else term!.write(new Uint8Array(ev.data))
  }
  ws.onclose = () => { status.value = '已断开' }
  ws.onerror = () => { status.value = '连接出错' }

  term.onData((d) => ws?.readyState === WebSocket.OPEN && ws.send(d))
  term.onResize(({ cols, rows }) => {
    if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: '1', cols, rows }))
  })
  window.addEventListener('resize', onResize)
})

function onResize() { fit?.fit() }

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  ws?.close()
  term?.dispose()
})
</script>

<template>
  <div class="page">
    <div class="toolbar">
      <el-tag :type="status === '已连接' ? 'success' : 'info'" size="small">{{ status }}</el-tag>
      <span class="mono">客户端 #{{ route.params.id }}</span>
    </div>
    <div ref="box" class="term-box" />
  </div>
</template>

<style scoped>
.term-box { height: calc(100vh - 180px); background: #0b1020; border-radius: 6px; padding: 8px; }
</style>
