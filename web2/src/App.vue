<script setup lang="ts">
// 4.9.3 的暗色主题通过 <html data-theme="dark"> 生效（见其 app-loading 样式）
// 同时把 antd 切到暗色算法，否则表格/卡片仍是白底（观感与 4.9.3 不符）。
import { computed } from 'vue'
import { theme } from 'ant-design-vue'
import { isDark } from './stores/theme'

const saved = localStorage.getItem('vshell.theme')
if (saved === 'dark') document.documentElement.setAttribute('data-theme', 'dark')

const cfgTheme = computed(() => ({
  algorithm: isDark.value ? theme.darkAlgorithm : theme.defaultAlgorithm,
}))
</script>

<template>
  <!-- autoInsertSpaceInButton=false：中文两字按钮不再被插入空格（"清 屏" → "清屏"），
       既符合原版观感，也让自动化断言能按文案匹配。 -->
  <a-config-provider :auto-insert-space-in-button="false" :theme="cfgTheme">
    <router-view />
  </a-config-provider>
</template>
