import { http } from './client'
import type { PermissionPayload, PlatformPermissionNode } from '@/types'

/** 平台端权限点（菜单）可视化配置 */
export const permissionApi = {
  tree() {
    return http.get<PlatformPermissionNode[]>('/platform/permissions/tree')
  },
  create(payload: PermissionPayload) {
    return http.post<PlatformPermissionNode>('/platform/permissions', payload)
  },
  update(id: number, payload: PermissionPayload) {
    return http.put<PlatformPermissionNode>(`/platform/permissions/${id}`, payload)
  },
  remove(id: number) {
    return http.delete<null>(`/platform/permissions/${id}`)
  },
}
