/* eslint-disable react-hooks/refs --
 * keep-alive 的实现必须在渲染期把 useOutlet() 的元素镜像进缓存 Map：
 * outlet 元素每次渲染都是新引用，无法用 state 做幂等更新（会触发无限重渲染）。
 * 此处的 ref 读写是刻意的「渲染期镜像」，写入值完全由当前路由输出决定，幂等且无外部副作用。
 */
import { Fragment, type ReactNode, useLayoutEffect, useMemo, useRef } from 'react'
import { useLocation, useOutlet } from 'react-router-dom'
import NProgress from 'nprogress'
import { findRouteByPath } from '@/router/helpers'
import { appConfig } from '@/config'
import { usePreferencesStore } from '@/store/preferences'
import { useTabsStore } from '@/store/tabs'

/**
 * 多标签页 KeepAlive 容器（React 无原生 keep-alive 的工程化折中）：
 * - 命中缓存的路由：已打开页面的元素常驻，非活动页以 display:none 隐藏，
 *   切回时组件实例、表单输入、滚动位置得以保留；
 * - 关闭页签 / 被权限裁剪时同步销毁缓存；
 * - refreshTicks[path] 变化会通过 key 强制整页 remount（页签「刷新」）；
 * - 偏好总开关 keepAliveEnable 关闭、或路由 meta.keepAlive === false 时直接渲染。
 *
 * 已知限制：缓存中的组件读取 react-router 的 location/params 时拿到的是当前 URL。
 * 当前业务页均为无路径参数的列表页（ProTable 内部状态驱动），不受影响；
 * 未来带 :id 的详情页请在路由 meta 中设置 keepAlive:false。
 */
export function KeepAliveOutlet() {
  const { pathname } = useLocation()
  const outlet = useOutlet()
  const cacheRef = useRef(new Map<string, ReactNode>())

  const keepAliveEnable = usePreferencesStore((s) => s.keepAliveEnable)
  // 注意：zustand v5 选择器返回值必须保持引用稳定，不能直接 s.tabs.map(...)，
  // 否则 useSyncExternalStore 会判定快照每次都变 → 无限重渲染
  const tabs = useTabsStore((s) => s.tabs)
  const openPaths = useMemo(() => tabs.map((tab) => tab.path), [tabs])
  const ticks = useTabsStore((s) => s.refreshTicks)

  const route = findRouteByPath(pathname)
  const cacheHit = cacheRef.current.has(pathname)
  const currentCacheable = keepAliveEnable && route?.keepAlive !== false

  if (!keepAliveEnable) cacheRef.current.clear()

  // 渲染期镜像：当前可缓存的 outlet 写入缓存；不可缓存路由移除其缓存
  if (currentCacheable && outlet) {
    cacheRef.current.set(pathname, outlet)
  } else if (!currentCacheable) {
    cacheRef.current.delete(pathname)
  }

  // 页签关闭 / 权限变化后，销毁不在打开列表中的缓存实例
  for (const cachedPath of cacheRef.current.keys()) {
    if (cachedPath !== pathname && !openPaths.includes(cachedPath)) {
      cacheRef.current.delete(cachedPath)
    }
  }

  useLayoutEffect(() => {
    NProgress.start()
    document.title = route ? `${route.title} · ${appConfig.title}` : appConfig.title

    // 命中缓存：无 chunk 加载，下一帧结束进度条；未命中由 RouteTracker 在页面挂载后结束
    let raf2 = 0
    if (cacheHit) {
      const raf1 = requestAnimationFrame(() => {
        raf2 = requestAnimationFrame(() => NProgress.done())
      })
      return () => cancelAnimationFrame(raf1)
    }
    // 兜底：异常情况下 4s 强制收起进度条
    const safety = window.setTimeout(() => NProgress.done(), 4000)
    return () => {
      cancelAnimationFrame(raf2)
      window.clearTimeout(safety)
    }
  }, [pathname, cacheHit, route])

  return (
    <Fragment>
      {[...cacheRef.current.entries()].map(([path, node]) => (
        <div
          key={`${path}:${ticks[path] ?? 0}`}
          className="app-page-fade"
          style={{ display: path === pathname ? 'block' : 'none', height: '100%' }}
        >
          {node}
        </div>
      ))}
      {!currentCacheable && (
        <div key={`${pathname}:${ticks[pathname] ?? 0}`} className="app-page-fade">
          {outlet}
        </div>
      )}
    </Fragment>
  )
}
