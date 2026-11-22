<script setup lang="ts">
// 4.9.3 的暗色主题通过 <html data-theme="dark"> 生效（见其 app-loading 样式）
// 同时把 antd 切到暗色算法，否则表格/卡片仍是白底（观感与 4.9.3 不符）。
import { computed } from 'vue'
import { theme } from 'ant-design-vue'
// 4.9.3 的 antd 文案是中文（bundle 里能看到默认校验文案「请输入${label}」、
// 空状态「暂无数据」），所以这里显式挂 zh_CN locale。
import zhCN from 'ant-design-vue/es/locale/zh_CN'
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
  <a-config-provider :auto-insert-space-in-button="false" :theme="cfgTheme" :locale="zhCN">
    <router-view />
  </a-config-provider>
</template>
