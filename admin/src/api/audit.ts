import { http } from './client'
import type { AuditLogItem, AuditLogQuery, PageResult } from '@/types'

/** 平台端审计日志（只读） */
export const auditApi = {
  list(params: AuditLogQuery) {
    return http.get<PageResult<AuditLogItem>>('/platform/audit-logs', params)
  },
}
