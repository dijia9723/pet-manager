import { http } from './client'
import type { PermissionNode, Role, RolePayload } from '@/types'

/** 租户端角色与权限点（RBAC） */
export const rbacApi = {
  listRoles() {
    return http.get<Role[]>('/tenant/roles')
  },
  createRole(payload: RolePayload) {
    return http.post<Role>('/tenant/roles', payload)
  },
  updateRole(id: number, payload: RolePayload) {
    return http.put<Role>(`/tenant/roles/${id}`, payload)
  },
  removeRole(id: number) {
    return http.delete<null>(`/tenant/roles/${id}`)
  },
  /** 权限点菜单树（角色授权树形勾选） */
  permissions() {
    return http.get<PermissionNode[]>('/tenant/permissions')
  },
}
