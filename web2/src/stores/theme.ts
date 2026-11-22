// 主题状态（与 4.9.3 一致：通过 <html data-theme="dark"> 生效）
// 这里额外把状态做成响应式，供 antd v-config-provider 切换到暗色算法。
import { ref } from 'vue'

export const isDark = ref(document.documentElement.getAttribute('data-theme') === 'dark')

export function setDark(v: boolean) {
  isDark.value = v
  document.documentElement.setAttribute('data-theme', v ? 'dark' : 'light')
  localStorage.setItem('vshell.theme', v ? 'dark' : 'light')
}

export function toggleDark() {
  setDark(!isDark.value)
}
