import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '@/store/auth'
import { usePermission } from '@/hooks/usePermission'

interface AuthGuardProps {
  /** 需要的单个权限点 */
  perm?: string
  /** 满足其中任意一个权限点即可 */
  anyOf?: string[]
  children: ReactNode
}

/**
 * 路由守卫：
 * 1. 未登录（无 access_token）跳登录页，并记录来源路径；
 * 2. 已登录但缺少权限点跳 403。
 */
export function AuthGuard({ perm, anyOf, children }: AuthGuardProps) {
  const location = useLocation()
  const accessToken = useAuthStore((s) => s.accessToken)
  const { has, hasAny } = usePermission()

  if (!accessToken) {
    return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  }

  const allowed = (perm ? has(perm) : true) && (anyOf && anyOf.length > 0 ? hasAny(anyOf) : true)
  if (!allowed) {
    return <Navigate to="/403" replace />
  }

  return <>{children}</>
}
