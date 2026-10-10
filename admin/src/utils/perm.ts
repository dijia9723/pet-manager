/**
 * 权限点判断工具。
 * 支持通配：`*` 代表拥有全部权限；`platform.*` 代表某前缀下的全部权限。
 */

export function hasPermission(permissions: string[], code?: string): boolean {
  if (!code) return true
  if (!permissions || permissions.length === 0) return false
  if (permissions.includes('*')) return true
  if (permissions.includes(code)) return true
  // 前缀通配：platform.* 可匹配 platform.tenant.list
  return permissions.some((p) => p.endsWith('.*') && code.startsWith(p.slice(0, -1)))
}

export function hasAnyPermission(permissions: string[], codes: string[]): boolean {
  return codes.some((code) => hasPermission(permissions, code))
}

export function hasAllPermissions(permissions: string[], codes: string[]): boolean {
  return codes.every((code) => hasPermission(permissions, code))
}
