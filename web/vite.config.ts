import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'
import UnoCSS from 'unocss/vite'

// Hub 管理面板独立部署（base 为根路径）。
// dev / preview 统一把 /api 代理到本地 Hub 后端 :8091（Go 服务端口）。
// 项目组端口约定：主仓库 web :5173、官网 :5174、SSO :5175，Hub 用 :5176，便于多站并行开发。
export default defineConfig(({ mode }) => ({
  base: '/',
  // devtools 仅开发启用：生产构建不应包含它（体积与运行时开销）
  plugins: [vue(), UnoCSS(), ...(mode === 'development' ? [vueDevTools()] : [])],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5176,
    proxy: {
      '/api': 'http://localhost:8091',
    },
  },
  // preview（生产构建预览）默认不带代理，导致本地跑 preview 时接口全 404；与 dev 保持一致
  preview: {
    proxy: {
      '/api': 'http://localhost:8091',
    },
  },
}))
