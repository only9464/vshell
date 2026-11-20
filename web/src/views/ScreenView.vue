<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import pako from 'pako'

// 屏幕直播：连 /api/screen/ws?id=<clientID>&quality=<big|normal|small>&token=<jwt>
// 服务端下发**二进制**帧（zlib 压缩的 PNG），前端 inflate 后塞进 <img>；
// 上行是 JSON 控制帧（见 docs/API.md §8.2）。
const route = useRoute()
const img = ref<HTMLImageElement>()
const status = ref('未连接')
const quality = ref<'big' | 'normal' | 'small'>('normal')
const fps = ref(0)
let ws: WebSocket | null = null
let frames = 0
let timer: number | undefined
let url: string | null = null

function connect() {
  close()
  const id = route.params.id
  const token = localStorage.getItem('vshell.token') || ''
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/screen/ws?id=${id}&quality=${quality.value}&token=${encodeURIComponent(token)}`)
  ws.binaryType = 'arraybuffer'
  ws.onopen = () => {
    status.value = '已连接'
    timer = window.setInterval(() => { fps.value = frames; frames = 0 }, 1000)
  }
  ws.onmessage = (ev) => {
    if (typeof ev.data === 'string') return // 控制帧
    try {
      const raw = pako.inflate(new Uint8Array(ev.data as ArrayBuffer))
      if (url) URL.revokeObjectURL(url)
      url = URL.createObjectURL(new Blob([new Uint8Array(raw).buffer as ArrayBuffer], { type: 'image/png' }))
      if (img.value) img.value.src = url
      frames++
    } catch (e) {
      // 非 zlib 帧：直接当图片处理（兼容未压缩帧）
      const raw = new Uint8Array(ev.data as ArrayBuffer)
      if (url) URL.revokeObjectURL(url)
      url = URL.createObjectURL(new Blob([new Uint8Array(raw).buffer as ArrayBuffer], { type: 'image/png' }))
      if (img.value) img.value.src = url
      frames++
    }
  }
  ws.onclose = () => { status.value = '已断开'; stopTimer() }
  ws.onerror = () => { status.value = '连接出错' }
}

function stopTimer() {
  if (timer) { clearInterval(timer); timer = undefined }
}

function close() {
  stopTimer()
  ws?.close()
  ws = null
  if (url) { URL.revokeObjectURL(url); url = null }
}

// 鼠标事件 → 服务端约定的 JSON（type 1 移动 / 2-4 点击 / 7 滚轮 / 8 组合键）
function send(obj: Record<string, unknown>) {
  if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify(obj))
}

function onMove(e: MouseEvent) {
  const el = e.currentTarget as HTMLImageElement
  const rect = el.getBoundingClientRect()
  const x = Math.round(((e.clientX - rect.left) / rect.width) * el.naturalWidth)
  const y = Math.round(((e.clientY - rect.top) / rect.height) * el.naturalHeight)
  send({ type: '1', absX: x, absY: y, canvasWidth: el.naturalWidth, canvasHeight: el.naturalHeight })
}
function onClick(e: MouseEvent) {
  const type = e.button === 2 ? '4' : e.button === 1 ? '3' : '2'
  send({ type })
}
function onWheel(e: WheelEvent) {
  send({ type: '7', direction: e.deltaY > 0 ? 'down' : 'up', amount: Math.max(1, Math.round(Math.abs(e.deltaY) / 100)) })
}
function onKey(e: KeyboardEvent) {
  send({ type: '8', key: e.key, modifiers: [e.ctrlKey ? 'ctrl' : '', e.altKey ? 'alt' : '', e.shiftKey ? 'shift' : ''].filter(Boolean) })
}

onMounted(connect)
onBeforeUnmount(close)
</script>

<template>
  <div class="page">
    <div class="toolbar">
      <el-tag :type="status === '已连接' ? 'success' : 'info'" size="small">{{ status }}</el-tag>
      <el-select v-model="quality" style="width: 120px" @change="connect">
        <el-option label="高清" value="big" />
        <el-option label="普通" value="normal" />
        <el-option label="流畅" value="small" />
      </el-select>
      <el-button @click="connect">重连</el-button>
      <span class="mono">{{ fps }} fps</span>
      <span class="mono">客户端 #{{ route.params.id }}</span>
    </div>
    <div class="stage">
      <img ref="img" class="screen" alt="remote screen"
           @mousemove="onMove" @click="onClick" @contextmenu.prevent="onClick"
           @wheel.prevent="onWheel" @keydown="onKey" tabindex="0" />
    </div>
  </div>
</template>

<style scoped>
.stage { background: #0b1020; min-height: calc(100vh - 180px); display: flex; align-items: flex-start; justify-content: center; }
.screen { max-width: 100%; cursor: crosshair; }
</style>
