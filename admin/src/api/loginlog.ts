import { http } from './client'
import type { LoginLogItem, LoginLogQuery, PageResult } from '@/types'

/** 平台端登录日志（只读） */
export const loginLogApi = {
  list(params: LoginLogQuery) {
    return http.get<PageResult<LoginLogItem>>('/platform/login-logs', params)
  },
}
