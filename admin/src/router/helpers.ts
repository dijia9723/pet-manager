import type { AppRouteModule } from '@/router/types'
import { appRouteModules } from './routes'

/** path → 路由模块，便于页签 / 标题 / 缓存快速查找 */
export const appRouteMap = new Map<string, AppRouteModule>(
  appRouteModules.map((route) => [route.path, route]),
)

/**
 * 按当前 pathname 匹配路由模块：精确匹配优先，随后按目录前缀匹配（预留 /a/b/:id 场景）。
 * 403 / 404 等框架页返回 undefined。
 */
export function findRouteByPath(pathname: string): AppRouteModule | undefined {
  return (
    appRouteMap.get(pathname) ??
    appRouteModules.find((route) => pathname.startsWith(`${route.path}/`))
  )
}

export { appRouteModules }
