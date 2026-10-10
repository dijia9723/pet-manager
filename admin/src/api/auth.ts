import { http } from './client'
import type {
  ChangePasswordPayload,
  LoginPayload,
  LoginResult,
  MenuNode,
  ProfileUpdatePayload,
  UserInfo,
} from '@/types'

/** 认证相关接口 */
export const authApi = {
  /** 登录：平台管理员不传 tenant_code */
  login(payload: LoginPayload) {
    return http.post<LoginResult>('/auth/login', payload)
  },
  /** 刷新 token */
  refresh(refreshToken: string) {
    return http.post<LoginResult>('/auth/refresh', { refresh_token: refreshToken })
  },
  /** 登出 */
  logout() {
    return http.post<null>('/auth/logout')
  },
  /** 当前用户信息（含 permissions） */
  profile() {
    return http.get<UserInfo>('/auth/profile')
  },
  /** 修改个人资料（昵称），返回最新用户信息 */
  updateProfile(payload: ProfileUpdatePayload) {
    return http.put<UserInfo>('/auth/profile', payload)
  },
  /** 修改自己的密码；成功后全部会话失效，需重新登录 */
  changePassword(payload: ChangePasswordPayload) {
    return http.post<null>('/auth/change-password', payload)
  },
  /** 当前用户菜单树（后端已按权限过滤） */
  menus() {
    return http.get<MenuNode[]>('/auth/menus')
  },
}
