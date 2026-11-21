<script setup lang="ts">
// 4.9.3 的暗色主题通过 <html data-theme="dark"> 生效（见其 app-loading 样式）
const saved = localStorage.getItem('vshell.theme')
if (saved === 'dark') document.documentElement.setAttribute('data-theme', 'dark')
</script>

<template>
  <router-view />
</template>
