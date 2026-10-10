/**
 * 后端接口 DTO 类型定义（契约冻结版，禁止改动后端）。
 * 统一响应体：{ code: 0, message: 'ok', data: ... }，code === 0 为成功。
 */

/** 统一响应包装 */
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

/** 分页响应 data */
export interface PageResult<T> {
  list: T[]
  total: number
}

/** 分页请求 query */
export interface PageQuery {
  page?: number
  page_size?: number
}

export type UserType = 'platform' | 'tenant'

/** 当前登录用户 */
export interface UserInfo {
  id: number
  username: string
  nickname: string
  user_type: UserType
  tenant_id: number
  tenant_code: string
  tenant_name: string
  roles: string[]
  permissions: string[]
}

/** 登录 / 刷新返回 */
export interface LoginResult {
  access_token: string
  refresh_token: string
  expires_in: number
  user: UserInfo
}

export interface LoginPayload {
  username: string
  password: string
  /** 平台管理员登录时不填 */
  tenant_code?: string
}

/** 修改个人资料（PUT /auth/profile） */
export interface ProfileUpdatePayload {
  nickname: string
}

/** 修改自己的密码（POST /auth/change-password） */
export interface ChangePasswordPayload {
  old_password: string
  new_password: string
}

/** 菜单树节点（/auth/menus） */
export interface MenuNode {
  key: string
  title: string
  path: string
  icon?: string
  children?: MenuNode[]
}

/* ------------------------------- 平台端 ------------------------------- */

export type TenantStatus = 'enabled' | 'disabled' | 'deleted'

export interface Tenant {
  id: number
  name: string
  code: string
  contact_name: string
  contact_phone: string
  status: TenantStatus
  expire_at: string
  /** 待销毁租户的物理回收时间，仅 status=deleted 时有值 */
  purge_at: string | null
  created_at: string
}

export interface TenantCreateOwner {
  username: string
  password: string
  nickname: string
}

export interface TenantCreatePayload {
  name: string
  code: string
  contact_name: string
  contact_phone: string
  owner: TenantCreateOwner
}

export type TenantUpdatePayload = Omit<TenantCreatePayload, 'owner'>

export interface TenantListQuery extends PageQuery {
  keyword?: string
  status?: TenantStatus
}

/* ------------------------------- 租户端 ------------------------------- */

export interface TenantUser {
  id: number
  username: string
  nickname: string
  status: string
  role_ids: number[]
  role_names: string[]
  created_at: string
}

export interface TenantUserCreatePayload {
  username: string
  password: string
  nickname: string
  role_ids: number[]
}

export interface TenantUserUpdatePayload {
  nickname: string
  role_ids: number[]
  status: string
}

export interface TenantUserListQuery extends PageQuery {
  keyword?: string
}

export interface Role {
  id: number
  name: string
  code: string
  is_system: boolean
  remark: string
  permission_ids: number[]
  user_count: number
}

export interface RolePayload {
  name: string
  code: string
  remark: string
  permission_ids: number[]
}

export type PermissionType = 'menu' | 'button'

export interface PermissionNode {
  id: number
  parent_id: number
  code: string
  name: string
  type: PermissionType
  children?: PermissionNode[]
}

export interface TenantInfo {
  id: number
  name: string
  code: string
  contact_name: string
  contact_phone: string
  address: string
  expire_at: string
  status: string
}

export type TenantInfoUpdatePayload = Omit<TenantInfo, 'id' | 'code'>

/* ------------------------------- 审计日志 ------------------------------- */

/** 审计日志条目（/platform/audit-logs，只读） */
export interface AuditLogItem {
  id: number
  tenant_id: number
  user_type: string
  user_id: number
  username: string
  method: string
  path: string
  route: string
  status: number
  client_ip: string
  user_agent: string
  latency_ms: number
  request_id: string
  created_at: string
}

export interface AuditLogQuery extends PageQuery {
  tenant_id?: number
  user_id?: number
  username?: string
  method?: string
  path?: string
  status?: number
  start?: string
  end?: string
}

/* ---------------------------- 权限点（菜单配置） ---------------------------- */

/** 权限点配置树节点（/platform/permissions/tree），比角色授权树多出 scope/path/icon/sort */
export interface PlatformPermissionNode {
  id: number
  parent_id: number
  code: string
  name: string
  type: PermissionType
  scope: UserType
  path?: string
  icon?: string
  sort: number
  children?: PlatformPermissionNode[]
}

export interface PermissionPayload {
  parent_id: number
  code: string
  name: string
  type: PermissionType
  scope: UserType
  path?: string
  icon?: string
  sort: number
}

/* ------------------------------- 登录日志 ------------------------------- */

export type LoginStatus = 'success' | 'fail'

/** 登录日志条目（/platform/login-logs，只读） */
export interface LoginLogItem {
  id: number
  username: string
  user_type: UserType | ''
  tenant_id: number
  tenant_code: string
  client_ip: string
  user_agent: string
  status: LoginStatus
  message: string
  created_at: string
}

export interface LoginLogQuery extends PageQuery {
  username?: string
  user_type?: UserType
  tenant_id?: number
  status?: LoginStatus
  start?: string
  end?: string
}

/* ------------------------------- 参数配置 ------------------------------- */

export type ConfigValueType = 'string' | 'number' | 'bool' | 'json'

/** 参数配置条目（/platform/configs） */
export interface ConfigItem {
  id: number
  config_key: string
  name: string
  value: string
  group_key: string
  type: ConfigValueType
  remark: string
  status: string
  created_at: string
}

export interface ConfigPayload {
  config_key: string
  name: string
  value: string
  group_key: string
  type: ConfigValueType
  remark: string
  status: string
}

export interface ConfigQuery extends PageQuery {
  keyword?: string
  group_key?: string
  status?: string
}

/* ------------------------------- 字典管理 ------------------------------- */

/** 字典类型（/platform/dicts） */
export interface DictType {
  id: number
  code: string
  name: string
  remark: string
  status: string
  created_at: string
}

export interface DictTypePayload {
  code: string
  name: string
  remark: string
  status: string
}

export interface DictTypeQuery extends PageQuery {
  keyword?: string
  status?: string
}

/** 字典项（/platform/dicts/:id/items） */
export interface DictEntry {
  storage_bytes?: number
  id: number
  dict_id: number
  label: string
  value: string
  sort: number
  status: string
  remark: string
  created_at: string
}

export interface DictEntryPayload {
  storage_bytes?: number
  label: string
  value: string
  sort: number
  status: string
  remark: string
}

/* ------------------------------- 任务管理 ------------------------------- */

export type JobStatus = 'enabled' | 'disabled'
export type JobRunStatus = 'running' | 'success' | 'fail'
export type JobTrigger = 'schedule' | 'manual'
export type JobKind = 'system' | 'http' | 'shell'

export interface JobExecutionConfig {
  handler?: string
  url?: string
  method?: string
  headers?: Record<string, string>
  body?: string
  curl?: string
  script?: string
  script_path?: string
  args?: string[]
  work_dir?: string
}

export interface JobMethodCatalog {
  methods: Array<{ code: string; name: string; remark: string }>
  shell_available: boolean
}

export interface JobCreatePayload {
  code: string
  name: string
  kind: JobKind
  spec: string
  remark: string
  status: JobStatus
  timeout_seconds: number
  config: JobExecutionConfig
}

/** 定时任务（/platform/jobs） */
export interface JobItem {
  id: number
  code: string
  name: string
  kind: JobKind
  /** 5 段 cron：分 时 日 月 周 */
  spec: string
  status: JobStatus
  remark: string
  builtin: boolean
  config: JobExecutionConfig
  timeout_seconds: number
  last_run_at: string | null
  last_status: JobRunStatus | ''
  last_message: string
  next_run_at: string | null
  created_at: string
}

export interface JobUpdatePayload {
  spec: string
  remark: string
  name?: string
  kind?: JobKind
  config?: JobExecutionConfig
  timeout_seconds?: number
}

/** 任务执行日志（/platform/jobs/:id/logs） */
export interface JobLogItem {
  id: number
  job_id: number
  job_code: string
  trigger: JobTrigger
  status: JobRunStatus
  started_at: string
  finished_at: string | null
  cost_ms: number
  result: string
}

export interface JobLogQuery extends PageQuery {
  status?: JobRunStatus
  trigger?: JobTrigger
}
