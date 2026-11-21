<script setup lang="ts">
// 4.9.3 的暗色主题通过 <html data-theme="dark"> 生效（见其 app-loading 样式）
const saved = localStorage.getItem('vshell.theme')
if (saved === 'dark') document.documentElement.setAttribute('data-theme', 'dark')
</script>

<template>
  <!-- autoInsertSpaceInButton=false：中文两字按钮不再被插入空格（"清 屏" → "清屏"），
       既符合原版观感，也让自动化断言能按文案匹配。 -->
  <a-config-provider :auto-insert-space-in-button="false">
    <router-view />
  </a-config-provider>
</template>
