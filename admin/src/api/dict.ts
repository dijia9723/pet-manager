import { http } from './client'
import type {
  DictEntry,
  DictEntryPayload,
  DictType,
  DictTypePayload,
  DictTypeQuery,
  PageResult,
} from '@/types'

/** 平台端字典管理（字典类型 + 字典项） */
export const dictApi = {
  listTypes(params: DictTypeQuery) {
    return http.get<PageResult<DictType>>('/platform/dicts', params)
  },
  createType(payload: DictTypePayload) {
    return http.post<DictType>('/platform/dicts', payload)
  },
  updateType(id: number, payload: DictTypePayload) {
    return http.put<DictType>(`/platform/dicts/${id}`, payload)
  },
  removeType(id: number) {
    return http.delete<null>(`/platform/dicts/${id}`)
  },

  listEntries(dictId: number) {
    return http.get<DictEntry[]>(`/platform/dicts/${dictId}/items`)
  },
  createEntry(dictId: number, payload: DictEntryPayload) {
    return http.post<DictEntry>(`/platform/dicts/${dictId}/items`, payload)
  },
  updateEntry(dictId: number, itemId: number, payload: DictEntryPayload) {
    return http.put<DictEntry>(`/platform/dicts/${dictId}/items/${itemId}`, payload)
  },
  removeEntry(dictId: number, itemId: number) {
    return http.delete<null>(`/platform/dicts/${dictId}/items/${itemId}`)
  },
}

/** 租户端字典管理（字典类型 + 字典项，数据落租户 schema） */
export const tenantDictApi = {
  listTypes(params: DictTypeQuery) {
    return http.get<PageResult<DictType>>('/tenant/dicts', params)
  },
  createType(payload: DictTypePayload) {
    return http.post<DictType>('/tenant/dicts', payload)
  },
  updateType(id: number, payload: DictTypePayload) {
    return http.put<DictType>(`/tenant/dicts/${id}`, payload)
  },
  removeType(id: number) {
    return http.delete<null>(`/tenant/dicts/${id}`)
  },

  listEntries(dictId: number) {
    return http.get<DictEntry[]>(`/tenant/dicts/${dictId}/items`)
  },
  createEntry(dictId: number, payload: DictEntryPayload) {
    return http.post<DictEntry>(`/tenant/dicts/${dictId}/items`, payload)
  },
  updateEntry(dictId: number, itemId: number, payload: DictEntryPayload) {
    return http.put<DictEntry>(`/tenant/dicts/${dictId}/items/${itemId}`, payload)
  },
  removeEntry(dictId: number, itemId: number) {
    return http.delete<null>(`/tenant/dicts/${dictId}/items/${itemId}`)
  },
}
