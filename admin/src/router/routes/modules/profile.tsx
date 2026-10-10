import { lazy } from 'react'
import type { AppRouteModule } from '@/router/types'

/**
 * 个人中心：平台/租户账号通用，仅需登录无权限点要求。
 * hideInMenu：不进侧边栏，由顶栏头像下拉菜单进入。
 */
const routes: AppRouteModule[] = [
  {
    key: 'messages',
    path: '/messages',
    title: '消息中心',
    icon: 'notification',
    order: 3,
    component: lazy(() => import('@/pages/messages')),
  },
  {
    key: 'profile',
    path: '/profile',
    title: '个人中心',
    icon: 'user',
    order: 2,
    hideInMenu: true,
    component: lazy(() => import('@/pages/profile')),
  },
]

export default routes
