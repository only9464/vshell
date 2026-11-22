import { defineConfig, presetUno, presetAttributify, presetIcons } from 'unocss'

// 4.9.3 用的是 UnoCSS + Tailwind 兼容写法：presetUno 已内含 tailwind/windi 预设。
export default defineConfig({
  // 4.9.3 的仪表盘用的是 iconify 的 flat-ui / flat-color-icons 图标，这里接同款集合
  presets: [
    presetUno(),
    presetAttributify(),
    presetIcons({ scale: 1.2, warn: false }),
  ],
  // 菜单图标是动态拼接的类名（i-${m.icon}），UnoCSS 的静态扫描看不到，
  // 必须显式列入 safelist，否则不会生成对应的 SVG 背景。
  safelist: [
    'i-ant-design:home-outlined',
    'i-fa6-solid:desktop',
    'i-fa6-solid:headphones',
    'i-fa6-solid:download',
    'i-ant-design:swap-outlined',
    'i-grommet-icons:services',
    'i-ion:settings-outline',
    'i-grommet-icons:action',
  ],
  shortcuts: {
    'flex-col': 'flex flex-col',
    'flex-center': 'flex items-center justify-center',
    'flex-between': 'flex items-center justify-between',
  },
})
