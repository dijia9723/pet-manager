import { http } from './client'
import type {
  PageResult,
  TenantUser,
  TenantUserCreatePayload,
  TenantUserListQuery,
  TenantUserUpdatePayload,
} from '@/types'

/** 租户端子账号管理 */
export const userApi = {
  list(query: TenantUserListQuery) {
    return http.get<PageResult<TenantUser>>('/tenant/users', query)
  },
  create(payload: TenantUserCreatePayload) {
    return http.post<TenantUser>('/tenant/users', payload)
  },
  update(id: number, payload: TenantUserUpdatePayload) {
    return http.put<TenantUser>(`/tenant/users/${id}`, payload)
  },
  remove(id: number) {
    return http.delete<null>(`/tenant/users/${id}`)
  },
  resetPassword(id: number, password: string) {
    return http.post<null>(`/tenant/users/${id}/reset-password`, { password })
  },
}
