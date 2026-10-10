import { http } from './client'
import type {
  PageResult,
  Tenant,
  TenantCreatePayload,
  TenantInfo,
  TenantInfoUpdatePayload,
  TenantListQuery,
  TenantUpdatePayload,
} from '@/types'

/**
 * 租户模块：
 * - `/platform/tenants*` 平台端租户管理（需 platform 身份）
 * - `/tenant/info` 租户自身信息（需 tenant 身份）
 */
export const tenantApi = {
  list(query: TenantListQuery) {
    return http.get<PageResult<Tenant>>('/platform/tenants', query)
  },
  detail(id: number) {
    return http.get<Tenant>(`/platform/tenants/${id}`)
  },
  create(payload: TenantCreatePayload) {
    return http.post<Tenant>('/platform/tenants', payload)
  },
  update(id: number, payload: TenantUpdatePayload) {
    return http.put<Tenant>(`/platform/tenants/${id}`, payload)
  },
  enable(id: number) {
    return http.post<null>(`/platform/tenants/${id}/enable`)
  },
  disable(id: number) {
    return http.post<null>(`/platform/tenants/${id}/disable`)
  },
  /** 删除租户：进入销毁宽限期（默认 7 天），到期由系统任务物理回收 */
  remove(id: number) {
    return http.post<null>(`/platform/tenants/${id}/delete`)
  },
  /** 撤销删除：恢复为停用态，需手动启用 */
  restore(id: number) {
    return http.post<null>(`/platform/tenants/${id}/restore`)
  },
  /** 跳过宽限期，永久销毁单个租户；存储对象由后台清理。 */
  destroy(id: number, confirmCode: string) {
    return http.post<{ pending_resources: number }>(`/platform/tenants/${id}/destroy`, {
      confirm_code: confirmCode,
    })
  },

  /** 租户自身信息 */
  getInfo() {
    return http.get<TenantInfo>('/tenant/info')
  },
  updateInfo(payload: TenantInfoUpdatePayload) {
    return http.put<TenantInfo>('/tenant/info', payload)
  },
}
