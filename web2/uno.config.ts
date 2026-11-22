import { defineConfig, presetUno, presetAttributify, presetIcons } from 'unocss'

// 4.9.3 用的是 UnoCSS + Tailwind 兼容写法：presetUno 已内含 tailwind/windi 预设。
export default defineConfig({
  // 4.9.3 的仪表盘用的是 iconify 的 flat-ui / flat-color-icons 图标，这里接同款集合
  presets: [
    presetUno(),
    presetAttributify(),
    presetIcons({ scale: 1.2, warn: false }),
  ],
  shortcuts: {
    'flex-col': 'flex flex-col',
    'flex-center': 'flex items-center justify-center',
    'flex-between': 'flex items-center justify-between',
  },
})
