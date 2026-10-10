import { useMemo } from 'react'
import { useAuthStore } from '@/store/auth'
import { hasAllPermissions, hasAnyPermission, hasPermission } from '@/utils/perm'

/** 权限点判断 hook，供菜单 / 按钮 / 页面显隐使用 */
export function usePermission() {
  const permissions = useAuthStore((s) => s.permissions)

  return useMemo(
    () => ({
      permissions,
      has: (code?: string) => hasPermission(permissions, code),
      hasAny: (codes: string[]) => hasAnyPermission(permissions, codes),
      hasAll: (codes: string[]) => hasAllPermissions(permissions, codes),
    }),
    [permissions],
  )
}
