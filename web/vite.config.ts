import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

/**
 * Licode Web 前端。
 *
 * 产物为纯静态 SPA（单入口 index.html + assets/），由 Go 后端 go:embed
 * 内嵌，并经 scripts/sync-frontend.sh 同步到 internal/web/dist。
 *
 * - base 固定 '/'：后端把 /assets/ 直接映射到产物目录，路径不能带前缀；
 * - 不输出 sourcemap：产物入库，体积与源码泄露面都要小；
 * - manualChunks 拆出 vendor：应用代码改动频繁，vendor 保持长缓存。
 */
const BACKEND = process.env.LICODE_BACKEND ?? 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5273,
    proxy: {
      '/api': { target: BACKEND, changeOrigin: true },
      '/ws': { target: BACKEND, ws: true, changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules')) {
            if (id.includes('vue-router')) return 'router'
            return 'vendor'
          }
        },
      },
    },
  },
})
