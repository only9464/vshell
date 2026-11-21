import { defineConfig, presetUno, presetAttributify, presetIcons } from 'unocss'

// 4.9.3 用的是 UnoCSS + Tailwind 兼容写法：presetUno 已内含 tailwind/windi 预设。
export default defineConfig({
  presets: [presetUno(), presetAttributify(), presetIcons()],
  shortcuts: {
    'flex-col': 'flex flex-col',
    'flex-center': 'flex items-center justify-center',
    'flex-between': 'flex items-center justify-between',
  },
})
