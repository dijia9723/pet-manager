import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { clearTokens, setTokens } from '@/utils/token'
import { STORE_KEYS } from '@/constants'
import type { LoginResult, MenuNode, UserInfo } from '@/types'
import { lockOwner, useScreenLockStore } from './screenLock'

interface AuthState {
  /** 当前登录用户（含 roles / permissions） */
  user: UserInfo | null
  /** 扁平化后的权限点集合，便于快速判断 */
  permissions: string[]
  /** 后端按权限过滤后的菜单树 */
  menus: MenuNode[]
  accessToken: string | null
  refreshToken: string | null
  /** 登录 / 刷新后写入会话 */
  setSession: (result: LoginResult) => void
  /** 拉取到最新 profile 后同步 */
  setUser: (user: UserInfo) => void
  setMenus: (menus: MenuNode[]) => void
  clearSession: () => void
}

/**
 * 认证状态（zustand + persist）。
 * token 会持久化在此 store；同时镜像写入 utils/token，作为 axios 层读取 token 的来源，
 * 避免 axios 直接 import store 造成循环依赖。
 */
export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      user: null,
      permissions: [],
      menus: [],
      accessToken: null,
      refreshToken: null,

      setSession: (result) => {
        setTokens(result.access_token, result.refresh_token)
        useScreenLockStore.getState().authenticatedLogin(lockOwner(result.user))
        set({
          user: result.user,
          permissions: result.user.permissions ?? [],
          accessToken: result.access_token,
          refreshToken: result.refresh_token,
          menus: [],
        })
      },

      setUser: (user) => {
        set({ user, permissions: user.permissions ?? [] })
      },

      setMenus: (menus) => {
        set({ menus })
      },

      clearSession: () => {
        clearTokens()
        set({ user: null, permissions: [], menus: [], accessToken: null, refreshToken: null })
      },
    }),
    {
      name: STORE_KEYS.auth,
      partialize: (state) => ({
        user: state.user,
        permissions: state.permissions,
        menus: state.menus,
        accessToken: state.accessToken,
        refreshToken: state.refreshToken,
      }),
      onRehydrateStorage: () => (state) => {
        // 恢复持久化数据后，把 token 镜像回 utils/token
        if (state?.accessToken && state.refreshToken) {
          setTokens(state.accessToken, state.refreshToken)
        }
      },
    },
  ),
)

/** 登录后统一落地工作台（/dashboard 无权限点限制，平台端/租户端菜单均下发） */
export function homePathFor(user: UserInfo | null): string {
  if (!user) return '/login'
  return '/dashboard'
}
