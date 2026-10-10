import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import NProgress from 'nprogress'
import { appConfig } from '@/config'

interface RouteTrackerProps {
  /** 所属路由的绝对路径（静态来自路由表，避免 keep-alive 缓存时读到当前 location） */
  path: string
  /** 页面标题 */
  title: string
}

/**
 * 路由副作用追踪器：
 * 懒加载页面真正渲染（chunk 就绪、组件挂载）后结束顶部进度条。
 * 挂在 Suspense 内部，只有 chunk 加载完成才会挂载；
 * 被 keep-alive 隐藏时通过 path 比对忽略非自身路由的变化。
 */
export function RouteTracker({ path, title }: RouteTrackerProps) {
  const { pathname } = useLocation()

  useEffect(() => {
    if (pathname === path) {
      document.title = `${title} · ${appConfig.title}`
      NProgress.done()
    }
  }, [path, pathname, title])

  return null
}
