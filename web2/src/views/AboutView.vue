<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { get } from '../api/http'

const info = ref<Record<string, any>>({})
onMounted(async () => {
  const res: any = await get('/dashboard/info')
  info.value = res?.result || {}
})
</script>

<template>
  <a-card :bordered="false" title="关于">
    <a-descriptions :column="1" bordered size="small" style="max-width: 720px">
      <a-descriptions-item label="产品">VShell 管理平台（复刻研究）</a-descriptions-item>
      <a-descriptions-item label="版本">{{ info.version || '-' }}</a-descriptions-item>
      <a-descriptions-item label="Web 端口">{{ info.web_port || '-' }}</a-descriptions-item>
      <a-descriptions-item label="支持的传输">TCP（原生，首选）· HTTP · HTTPS · KCP · WebSocket · DNS</a-descriptions-item>
      <a-descriptions-item label="面板接口">61 个 API（JWT 鉴权，Token 头，24h 过期）</a-descriptions-item>
      <a-descriptions-item label="原生 MCP">stdio JSON-RPC，13 个工具，与面板同一套 JWT 鉴权</a-descriptions-item>
      <a-descriptions-item label="按需求不实现">通知（钉钉/微信）· OSS 上传下载 · 屏幕监控 · 屏幕截图 · 许可证</a-descriptions-item>
      <a-descriptions-item label="说明">
        基于二进制分析的独立重实现，仅用于授权环境的安全研究。
      </a-descriptions-item>
    </a-descriptions>
  </a-card>
</template>
