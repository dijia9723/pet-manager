import { http } from './client'
import type { PageResult } from '@/types'

export interface ManagedServer {
  id: number
  name: string
  host: string
  port: number
  username: string
  auth_type: 'key' | 'password'
  host_fingerprint: string
  status: 'enabled' | 'disabled'
  remark: string
  created_at: string
}
export type ServerPayload = Omit<ManagedServer, 'id' | 'created_at'> & { credential?: string }
export interface Website {
  id: number
  tenant_id: number
  server_id: number
  name: string
  url: string
  cms_url: string
  cms: 'none' | 'directus'
  status: 'enabled' | 'disabled'
  provision_status: 'pending' | 'provisioning' | 'ready' | 'failed'
  cms_status: 'pending' | 'deploying' | 'ready' | 'failed'
  cms_port: number
  cms_version: string
  remark: string
  created_at: string
}
export type WebsitePayload = Pick<
  Website,
  'tenant_id' | 'server_id' | 'name' | 'url' | 'cms_url' | 'cms' | 'status' | 'remark'
>
export interface FileItem {
  name: string
  size: number
  directory: boolean
  symlink: boolean
  modified_at: string
}
export interface CMSDeployPayload {
  port: number
  admin_email: string
  admin_password: string
  license_key?: string
}
export interface HostingQuery {
  page?: number
  page_size?: number
  keyword?: string
  tenant_id?: number
}
export const serverApi = {
  list: (query: HostingQuery) => http.get<PageResult<ManagedServer>>('/platform/servers', query),
  create: (data: ServerPayload) => http.post<ManagedServer>('/platform/servers', data),
  update: (id: number, data: ServerPayload) =>
    http.put<ManagedServer>(`/platform/servers/${id}`, data),
  remove: (id: number) => http.delete(`/platform/servers/${id}`),
  test: (id: number) =>
    http.post<{ output: string; exit_code: number }>(`/platform/servers/${id}/test`, undefined, {
      timeout: 35000,
    }),
  exec: (id: number, command: string) =>
    http.post<{ output: string; exit_code: number }>(
      `/platform/servers/${id}/exec`,
      { command },
      { timeout: 35000 },
    ),
}
export function websiteApi(scope: 'platform' | 'tenant') {
  const base = `/${scope}/websites`
  return {
    list: (query: HostingQuery) => http.get<PageResult<Website>>(base, query),
    create: (data: WebsitePayload) => http.post<Website>(base, data),
    update: (id: number, data: WebsitePayload) => http.put<Website>(`${base}/${id}`, data),
    provision: (id: number) => http.post(`${base}/${id}/provision`, undefined, { timeout: 65000 }),
    deploy: (id: number, data: CMSDeployPayload) =>
      http.post(`${base}/${id}/cms/deploy`, data, { timeout: 190000 }),
    files: (id: number, path: string) =>
      http.get<FileItem[]>(`${base}/${id}/files`, { path }, { timeout: 35000 }),
    read: (id: number, path: string) =>
      http.get<{ content: string }>(`${base}/${id}/files/content`, { path }, { timeout: 35000 }),
    write: (id: number, path: string, content: string) =>
      http.put(`${base}/${id}/files/content`, { path, content }, { timeout: 35000 }),
    download: (id: number, path: string) =>
      http.get<Blob>(
        `${base}/${id}/files/download`,
        { path },
        { responseType: 'blob', timeout: 35000 },
      ),
    upload: (id: number, path: string, file: File) => {
      const data = new FormData()
      data.append('path', path)
      data.append('file', file)
      return http.post(`${base}/${id}/files/upload`, data, { timeout: 35000 })
    },
    action: (
      id: number,
      operation: 'mkdir' | 'rename' | 'delete',
      path: string,
      destination?: string,
    ) =>
      http.post(
        `${base}/${id}/files/actions`,
        { operation, path, destination },
        { timeout: 35000 },
      ),
  }
}
