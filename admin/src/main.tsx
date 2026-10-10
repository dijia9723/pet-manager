import { StrictMode, useEffect, useLayoutEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider } from 'react-router-dom'
import { App as AntdApp, ConfigProvider, Empty, theme } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import 'antd/dist/reset.css'
import 'nprogress/nprogress.css'
import '@/styles/global.css'
import NProgress from 'nprogress'
import { router } from '@/router'
import { AppEmpty } from '@/components/AppEmpty'
import { bindMessageInstance, setUnauthorizedHandler } from '@/api/client'
import { useAuthStore } from '@/store/auth'
import { usePreferencesStore } from '@/store/preferences'
import { buildAppTheme, effectivePrimary } from '@/config/theme'

dayjs.locale('zh-cn')

NProgress.configure({ showSpinner: false, minimum: 0.15, trickleSpeed: 120 })

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 30_000,
    },
  },
})

// 401 兜底：清空本地会话并跳登录页
setUnauthorizedHandler(() => {
  useAuthStore.getState().clearSession()
  if (window.location.pathname !== '/login') {
    window.location.replace('/login')
  }
})

/** 把 App.useApp() 的 message 注入 axios 层，替代 antd 静态 message（消费主题上下文） */
function MessageBridge() {
  const { message } = AntdApp.useApp()
  useEffect(() => {
    bindMessageInstance(message)
  }, [message])
  return null
}

/** body / portal 外层不在 ConfigProvider 的变量作用域内，同步真实颜色供全局样式消费。 */
function ThemeVariablesBridge() {
  const { token } = theme.useToken()
  useLayoutEffect(() => {
    const root = document.documentElement
    for (const [key, value] of Object.entries(token)) {
      if (typeof value === 'string' && (key.startsWith('color') || key.startsWith('boxShadow'))) {
        root.style.setProperty(
          `--ant-${key.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`)}`,
          value,
        )
      }
    }
  }, [token])
  return null
}

/** 主题根：ConfigProvider 与 preferences store 联动（风格 / 明暗 / 主题色） */
function ThemedRoot() {
  const themeMode = usePreferencesStore((s) => s.themeMode)
  const themePreset = usePreferencesStore((s) => s.themePreset)
  const colorPrimary = usePreferencesStore((s) => s.colorPrimary)
  const isDark = themeMode === 'dark'
  const isCute = themePreset === 'cute'
  const themeOptions = { preset: themePreset, mode: themeMode, colorPrimary }
  const activePrimary = effectivePrimary(themeOptions)

  useLayoutEffect(() => {
    const root = document.documentElement
    root.classList.toggle('app-dark', isDark)
    root.classList.toggle('app-cute', isCute)
    root.setAttribute('data-theme-preset', themePreset)
    root.style.setProperty('--app-primary', activePrimary)
  }, [isDark, isCute, themePreset, activePrimary])

  return (
    <ConfigProvider
      locale={zhCN}
      theme={buildAppTheme(themeOptions)}
      // 表格空态用品牌爪印插画，其它组件保持 antd 默认空态
      renderEmpty={(componentName) => (componentName === 'Table' ? <AppEmpty /> : <Empty />)}
    >
      <AntdApp>
        <ThemeVariablesBridge />
        <MessageBridge />
        <QueryClientProvider client={queryClient}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </AntdApp>
    </ConfigProvider>
  )
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemedRoot />
  </StrictMode>,
)

// 首屏 loading 使命完成，移除占位节点
document.getElementById('app-loading')?.remove()
