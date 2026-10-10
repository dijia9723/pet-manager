import { http } from './client'
import type {
  JobCreatePayload,
  JobItem,
  JobLogItem,
  JobLogQuery,
  JobMethodCatalog,
  JobUpdatePayload,
  PageResult,
} from '@/types'

/**
 * 任务管理模块（/platform/jobs*）：系统定时任务的调度治理、手动执行与执行日志。
 */
export const jobApi = {
  methods() {
    return http.get<JobMethodCatalog>('/platform/jobs/methods')
  },
  create(payload: JobCreatePayload) {
    return http.post<JobItem>('/platform/jobs', payload)
  },
  remove(id: number) {
    return http.delete<null>(`/platform/jobs/${id}`)
  },
  list() {
    return http.get<JobItem[]>('/platform/jobs')
  },
  update(id: number, payload: JobUpdatePayload) {
    return http.put<null>(`/platform/jobs/${id}`, payload)
  },
  enable(id: number) {
    return http.post<null>(`/platform/jobs/${id}/enable`)
  },
  disable(id: number) {
    return http.post<null>(`/platform/jobs/${id}/disable`)
  },
  /** 立即执行一次（异步触发，结果在执行日志中查看） */
  run(id: number) {
    return http.post<null>(`/platform/jobs/${id}/run`)
  },
  logs(id: number, query: JobLogQuery) {
    return http.get<PageResult<JobLogItem>>(`/platform/jobs/${id}/logs`, query)
  },
}
