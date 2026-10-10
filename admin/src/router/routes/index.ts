import type { AppRouteModule } from '@/router/types'
import dashboard from './modules/dashboard'
import profile from './modules/profile'
import platform from './modules/platform'
import tenant from './modules/tenant'

/**
 * 全部业务路由模块聚合。
 * 新增页面时：在 modules/ 下对应文件（或新建文件并在此 import）追加一项即可。
 */
export const appRouteModules: AppRouteModule[] =
  [...dashboard, ...profile, ...platform, ...tenant].sort(
    (a, b) => (a.order ?? 0) - (b.order ?? 0),
  )
