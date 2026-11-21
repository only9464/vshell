/** Tailwind 与 UnoCSS 并存：4.9.3 的样式里两类原子类都有。 */
export default {
  content: ['./index.html', './src/**/*.{vue,ts,tsx}'],
  darkMode: ['class', '[data-theme="dark"]'],
  corePlugins: { preflight: false }, // 与 ant-design-vue 的 reset 冲突，关掉
  theme: { extend: {} },
  plugins: [],
}
