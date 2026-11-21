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
      <a-descriptions-item label="许可到期">{{ info.licTime || '-' }}</a-descriptions-item>
      <a-descriptions-item label="Web 端口">{{ info.web_port || '-' }}</a-descriptions-item>
      <a-descriptions-item label="客户端上限">{{ info.clientNum || '-' }}</a-descriptions-item>
      <a-descriptions-item label="说明">
        基于二进制分析的独立重实现，仅用于授权环境的安全研究。
      </a-descriptions-item>
    </a-descriptions>
  </a-card>
</template>
