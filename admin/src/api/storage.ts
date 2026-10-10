import { http } from './client'
import type { DictEntry, PageQuery, PageResult, UserType } from '@/types'

export const GiB = 1024 ** 3
export function formatBytes(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${Number((bytes / 1024 ** i).toFixed(2))} ${units[i]}`
}
export interface StorageResource {
  id: number
  tenant_id: number
  tenant_name: string
  name: string
  size_bytes: number
  content_type: string
  status: 'uploading' | 'ready' | 'deleting'
  uploader_name: string
  created_at: string
}
export interface Attachment {
  resource_id: number
  name: string
  size_bytes: number
  content_type: string
}
export interface StorageSummary {
  tenant_id: number
  plan_id: number
  plan_name: string
  base_bytes: number
  extra_bytes: number
  used_bytes: number
  quota_bytes: number
  unlimited: boolean
  enabled: boolean
  max_upload_bytes: number
}
export interface StorageAccount extends StorageSummary {
  tenant_name: string
  tenant_code: string
}
export interface StorageChange {
  id: number
  action: 'assign' | 'expand'
  plan_name: string
  before_bytes: number
  after_bytes: number
  operator_name: string
  remark: string
  created_at: string
}
export interface StorageQuery extends PageQuery {
  keyword?: string
  tenant_id?: number
  status?: string
  own?: boolean
}
export const storageApi = (scope: UserType) => ({
  summary: () => http.get<StorageSummary>(`/${scope}/storage/summary`),
  list: (query: StorageQuery) =>
    http.get<PageResult<StorageResource>>(`/${scope}/storage/resources`, query),
  upload: (file: File) => {
    const data = new FormData()
    data.append('file', file)
    return http.post<StorageResource>(`/${scope}/storage/resources`, data, { timeout: 180000 })
  },
  remove: (id: number) => http.delete(`/${scope}/storage/resources/${id}`),
  download: (id: number) => http.get<{ url: string }>(`/${scope}/storage/resources/${id}/download`),
})
export const storagePlatformApi = {
  plans: () => http.get<DictEntry[]>('/platform/storage/plans'),
  accounts: (query: StorageQuery) =>
    http.get<PageResult<StorageAccount>>('/platform/storage/accounts', query),
  allocate: (
    id: number,
    input: { action: 'assign' | 'expand'; plan_id?: number; add_bytes?: number; remark?: string },
  ) => http.put<StorageSummary>(`/platform/storage/accounts/${id}`, input),
  changes: (id: number, query: PageQuery) =>
    http.get<PageResult<StorageChange>>(`/platform/storage/accounts/${id}/changes`, query),
}

export function openDownload(url: string) {
  const link = document.createElement('a')
  link.href = url
  link.target = '_blank'
  link.rel = 'noopener noreferrer'
  link.click()
}
