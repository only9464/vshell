import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'
import 'element-plus/theme-chalk/dark/css-vars.css'
import '@xterm/xterm/css/xterm.css'
import './styles/main.css'
import App from './App.vue'
import router from './router'

// 恢复上次的主题选择（暗色：给 <html> 加 .dark，Element Plus 的 CSS 变量随之切换）
if (localStorage.getItem('vshell.theme') === 'dark') {
  document.documentElement.classList.add('dark')
}

createApp(App).use(createPinia()).use(router).use(ElementPlus, { locale: zhCn }).mount('#app')
