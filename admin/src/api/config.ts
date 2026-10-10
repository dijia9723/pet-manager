import { http } from './client'
import type { ConfigItem, ConfigPayload, ConfigQuery, PageResult } from '@/types'

/** 平台端参数配置 */
export const configApi = {
  list(params: ConfigQuery) {
    return http.get<PageResult<ConfigItem>>('/platform/configs', params)
  },
  create(payload: ConfigPayload) {
    return http.post<ConfigItem>('/platform/configs', payload)
  },
  update(id: number, payload: ConfigPayload) {
    return http.put<ConfigItem>(`/platform/configs/${id}`, payload)
  },
  remove(id: number) {
    return http.delete<null>(`/platform/configs/${id}`)
  },
}

/** 租户端参数配置（数据落租户 schema，与平台参数互不相通） */
export const tenantConfigApi = {
  list(params: ConfigQuery) {
    return http.get<PageResult<ConfigItem>>('/tenant/configs', params)
  },
  create(payload: ConfigPayload) {
    return http.post<ConfigItem>('/tenant/configs', payload)
  },
  update(id: number, payload: ConfigPayload) {
    return http.put<ConfigItem>(`/tenant/configs/${id}`, payload)
  },
  remove(id: number) {
    return http.delete<null>(`/tenant/configs/${id}`)
  },
}
