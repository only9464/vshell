import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发时把 /api 代理到 Go 服务端（默认 :8082）；产物输出到 web/dist，
// 由服务端 /app/ 路径托管（见 docs/FRONTEND.md）。
export default defineConfig({
  plugins: [vue()],
  base: '/app/',
  build: { outDir: 'dist', emptyOutDir: true },
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://127.0.0.1:8082', changeOrigin: true, ws: true },
    },
  },
})
