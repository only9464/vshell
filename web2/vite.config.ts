import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'

// 对齐 4.9.3 前端技术栈：Vue3 + Ant Design Vue + UnoCSS + Tailwind。
// 构建产物输出到 ../web2/dist，由 Go 服务端在 /app/ 下托管。
export default defineConfig({
  base: '/app/',
  plugins: [vue(), UnoCSS()],
  build: { outDir: 'dist', emptyOutDir: true },
  server: {
    proxy: { '/api': { target: 'http://127.0.0.1:8082', changeOrigin: true } },
  },
})
