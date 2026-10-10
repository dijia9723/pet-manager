import { lazy } from 'react'
import type { AppRouteModule } from '@/router/types'

/** 通用页面（不区分平台端/租户端） */
const routes: AppRouteModule[] = [
  {
    key: 'dashboard',
    path: '/dashboard',
    title: '工作台',
    icon: 'dashboard',
    order: 1,
    component: lazy(() => import('@/pages/dashboard')),
  },
]

export default routes
