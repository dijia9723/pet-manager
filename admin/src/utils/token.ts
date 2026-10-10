/**
 * Token 本地存取工具。
 * 作为 axios 层读取 token 的唯一来源，避免与 zustand store 形成循环依赖。
 * 键名带命名空间前缀，见 constants/storageKey。
 */
import { storageKey } from '@/constants'

const ACCESS_TOKEN_KEY = storageKey('access-token')
const REFRESH_TOKEN_KEY = storageKey('refresh-token')

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_TOKEN_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY)
}

export function setTokens(accessToken: string, refreshToken: string): void {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken)
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}
