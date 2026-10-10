import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import { sep } from 'node:path'
import { fileURLToPath } from 'node:url'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  return {
    base: env.VITE_BASE || '/',
    plugins: [react()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      port: Number(env.VITE_PORT) || 5173,
      host: '0.0.0.0',
      proxy: {
        // 后端 Gin 服务，目标地址来自 .env.development 的 VITE_PROXY_TARGET
        '/api': {
          target: env.VITE_PROXY_TARGET || 'http://127.0.0.1:8088',
          changeOrigin: true,
          ws: true,
        },
      },
    },
    build: {
      // ProComponents 单包体积较大但长期独立缓存，中后台场景可接受
      chunkSizeWarningLimit: 2000,
      rollupOptions: {
        output: {
          // 第三方依赖手动分包：长期缓存 + 避免单包过大
          manualChunks(id) {
            if (!id.includes('node_modules')) return
            // ProComponents 全家桶（pro-table/pro-form 等，体积大，单独长期缓存）
            // 必须在 antd 规则之前匹配：其包名为 @ant-design/pro-*
            if (id.includes('@ant-design/pro-')) return 'pro-vendor'
            // antd 内核（含 icons / cssinjs / rc-* / @rc-component；
            // icons 依赖 antd 的 cssinjs，合并以避免循环 chunk）
            if (
              id.includes(`${sep}antd${sep}`) ||
              id.includes(`${sep}rc-`) ||
              id.includes('@rc-component') ||
              id.includes('@ant-design/icons') ||
              id.includes('@ant-design/cssinjs')
            ) {
              return 'antd-vendor'
            }
            // React 内核与主流状态/路由库
            if (
              id.includes('react-router') ||
              id.includes('@tanstack') ||
              id.includes('zustand') ||
              id.includes(`${sep}react${sep}`) ||
              id.includes('react-dom') ||
              id.includes('scheduler')
            ) {
              return 'react-vendor'
            }
            if (id.includes('axios') || id.includes('dayjs') || id.includes('nprogress')) {
              return 'utils-vendor'
            }
          },
        },
      },
    },
  }
})
