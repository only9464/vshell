<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { dashboardApi } from '../api'

const info = ref<Record<string, any>>({})
onMounted(async () => { info.value = await dashboardApi.info() })
</script>

<template>
  <div class="page">
    <el-card style="max-width: 720px">
      <template #header>关于</template>
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item label="服务端版本">{{ info.version || '-' }}</el-descriptions-item>
        <el-descriptions-item label="面板">{{ 'Vue3 复刻前端（web/）' }}</el-descriptions-item>
        <el-descriptions-item label="版本类型">复刻研究版（许可证功能未实现）</el-descriptions-item>
        <el-descriptions-item label="说明">
          本项目是对 VShell v3.0/4.9.3 的独立逆向复刻研究，与原版作者无关，仅供授权安全研究。
        </el-descriptions-item>
        <el-descriptions-item label="文档">docs/ 目录：ROADMAP / STATUS / PROTOCOL / API / REFERENCE</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>
