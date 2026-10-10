import { http } from './client'
import type { Attachment } from './storage'
import type { PageQuery, PageResult, UserType } from '@/types'

export type TargetType =
  'all_tenants' | 'tenants' | 'tenant_users' | 'platform_users' | 'own_tenant'
export interface NotificationInput {
  attachment_ids?: number[]
  title: string
  content: string
  category: 'announcement' | 'maintenance' | 'business'
  priority: 'normal' | 'urgent'
  require_process: boolean
  target_type: TargetType
  target_ids: number[]
}
export interface NotificationItem extends NotificationInput {
  attachments?: Attachment[] | null
  id: number
  status: 'draft' | 'published' | 'revoked'
  sender_name: string
  sender_type: UserType
  sender_tenant_id: number
  created_at: string
  published_at: string | null
  recipient_count: number
  read_count: number
  processed_count: number
}
export interface InboxItem extends NotificationItem {
  read_at: string | null
  processed_at: string | null
  feedback: string
}
export interface Receipt {
  id: number
  user_type: UserType
  tenant_id: number
  username: string
  tenant_name: string
  read_at: string | null
  processed_at: string | null
  feedback: string
}
export interface NotificationQuery extends PageQuery {
  keyword?: string
  status?: string
  state?: string
}
export const createNotificationApi = (type: UserType) => ({
  list: (query: NotificationQuery) =>
    http.get<PageResult<NotificationItem>>(`/${type}/notifications`, query),
  save: (input: NotificationInput, id?: number) =>
    id
      ? http.put<NotificationItem>(`/${type}/notifications/${id}`, input)
      : http.post<NotificationItem>(`/${type}/notifications`, input),
  publish: (id: number) => http.post(`/${type}/notifications/${id}/publish`),
  revoke: (id: number) => http.post(`/${type}/notifications/${id}/revoke`),
  receipts: (id: number, query: NotificationQuery) =>
    http.get<PageResult<Receipt>>(`/${type}/notifications/${id}/receipts`, query),
  targets: (
    targetType: Exclude<TargetType, 'all_tenants' | 'own_tenant'>,
    keyword = '',
    page = 1,
  ) =>
    http.get<PageResult<{ id: number; label: string }>>(`/${type}/notifications/targets`, {
      type: targetType,
      keyword,
      page,
      page_size: 50,
    }),
})
export const messageApi = (type: UserType) => ({
  list: (query: NotificationQuery) => http.get<PageResult<InboxItem>>(`/${type}/messages`, query),
  unread: () => http.get<number>(`/${type}/messages/unread-count`),
  read: (id: number) => http.post(`/${type}/messages/${id}/read`),
  process: (id: number, feedback: string) =>
    http.post(`/${type}/messages/${id}/process`, { feedback }),
})
