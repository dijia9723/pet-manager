import { lazy } from 'react'
import type { AppRouteModule } from '@/router/types'

/** 租户端页面 */
const routes: AppRouteModule[] = [
  {
    key: 'tenant.website.list',
    path: '/tenant/websites',
    title: '网站管理',
    icon: 'website',
    perm: 'tenant.website.list',
    order: 40,
    component: lazy(() => import('@/pages/tenant/websites')),
  },
  {
    key: 'tenant.storage.list',
    path: '/tenant/storage',
    title: '存储资源',
    icon: 'storage',
    perm: 'tenant.storage.list',
    order: 39,
    component: lazy(() => import('@/pages/tenant/storage')),
  },
  {
    key: 'tenant.user.list',
    path: '/tenant/users',
    title: '子账号管理',
    icon: 'team',
    perm: 'tenant.user.list',
    order: 20,
    component: lazy(() => import('@/pages/tenant/users')),
  },
  {
    key: 'tenant.role.list',
    path: '/tenant/roles',
    title: '角色授权',
    icon: 'safety',
    perm: 'tenant.role.list',
    order: 21,
    component: lazy(() => import('@/pages/tenant/roles')),
  },
  {
    key: 'tenant.param.list',
    path: '/tenant/configs',
    title: '参数配置',
    icon: 'config',
    perm: 'tenant.param.list',
    order: 30,
    component: lazy(() => import('@/pages/tenant/configs')),
  },
  {
    key: 'tenant.dict.list',
    path: '/tenant/dicts',
    title: '字典管理',
    icon: 'appstore',
    perm: 'tenant.dict.list',
    order: 31,
    component: lazy(() => import('@/pages/tenant/dicts')),
  },
]

export default routes
