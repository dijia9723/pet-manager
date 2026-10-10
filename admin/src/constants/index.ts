import { appConfig } from '@/config'

/**
 * 本地缓存常量：
 * 所有 localStorage / zustand persist 的键都通过本文件生成，
 * 以 VITE_APP_NAMESPACE 做前缀隔离（同域名部署多个应用互不污染）。
 */
export const STORAGE_PREFIX = appConfig.namespace

export function storageKey(key: string): string {
  return `${STORAGE_PREFIX}:${key}`
}

/** zustand persist 存储键 */
export const STORE_KEYS = {
  auth: storageKey('auth'),
  preferences: storageKey('preferences'),
  tabs: storageKey('tabs'),
  screenLock: storageKey('screen-lock'),
} as const
