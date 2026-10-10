import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { STORE_KEYS } from '@/constants'
import type { UserInfo } from '@/types'

interface LockAccount {
  salt: string
  passwordHash: string
  locked: boolean
  failedAttempts: number
  retryAt: number
}

interface ScreenLockState {
  accounts: Record<string, LockAccount>
  setPassword: (owner: string, password: string, currentPassword?: string) => Promise<void>
  lock: (owner: string) => void
  unlock: (owner: string, password: string) => Promise<boolean>
  authenticatedLogin: (owner: string) => void
}

export function lockOwner(user: UserInfo | null): string {
  return user ? `${user.user_type}:${user.tenant_id}:${user.id}` : ''
}

async function hashPassword(password: string, salt: string): Promise<string> {
  if (!globalThis.crypto?.subtle) {
    throw new Error('当前环境无法安全保存锁屏密码，请使用 HTTPS 或 localhost 访问')
  }
  const key = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(password),
    'PBKDF2',
    false,
    ['deriveBits'],
  )
  const bits = await crypto.subtle.deriveBits(
    {
      name: 'PBKDF2',
      hash: 'SHA-256',
      iterations: 210_000,
      salt: Uint8Array.from(salt.match(/.{2}/g) ?? [], (byte) => parseInt(byte, 16)),
    },
    key,
    256,
  )
  return Array.from(new Uint8Array(bits), (byte) => byte.toString(16).padStart(2, '0')).join('')
}

export const useScreenLockStore = create<ScreenLockState>()(
  persist(
    (set, get) => {
      const update = (owner: string, patch: Partial<LockAccount>) =>
        set((state) => ({
          accounts: { ...state.accounts, [owner]: { ...state.accounts[owner], ...patch } },
        }))

      const verify = async (owner: string, password: string): Promise<boolean> => {
        const account = get().accounts[owner]
        if (!account) return false
        if (account.retryAt > Date.now()) {
          throw new Error(
            `密码错误次数过多，请 ${Math.ceil((account.retryAt - Date.now()) / 1000)} 秒后再试`,
          )
        }
        const passwordHash = await hashPassword(password, account.salt)
        // 异步计算期间另一标签页可能修改了密码，不能用旧摘要解锁。
        if (get().accounts[owner]?.passwordHash !== account.passwordHash) return false
        if (passwordHash === account.passwordHash) {
          update(owner, { failedAttempts: 0, retryAt: 0 })
          return true
        }
        const attempts = (get().accounts[owner]?.failedAttempts ?? 0) + 1
        update(owner, {
          failedAttempts: attempts >= 5 ? 0 : attempts,
          retryAt: attempts >= 5 ? Date.now() + 30_000 : 0,
        })
        if (attempts >= 5) throw new Error('密码错误次数过多，请 30 秒后再试')
        return false
      }

      return {
        accounts: {},
        setPassword: async (owner, password, currentPassword = '') => {
          if (!owner) throw new Error('请先登录')
          if (password.length < 6 || password.length > 64 || !password.trim()) {
            throw new Error('锁屏密码需为 6–64 位，不能全为空格')
          }
          const existing = get().accounts[owner]
          if (existing && !(await verify(owner, currentPassword)))
            throw new Error('当前锁屏密码不正确')
          const salt = Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) =>
            byte.toString(16).padStart(2, '0'),
          ).join('')
          const passwordHash = await hashPassword(password, salt)
          // 防止计算期间覆盖另一标签页刚保存的密码。
          if (get().accounts[owner]?.passwordHash !== existing?.passwordHash) {
            throw new Error('锁屏密码已在其他页面更新，请重试')
          }
          update(owner, {
            salt,
            passwordHash,
            locked: get().accounts[owner]?.locked ?? false,
            failedAttempts: 0,
            retryAt: 0,
          })
        },
        lock: (owner) => {
          if (get().accounts[owner]) update(owner, { locked: true })
        },
        unlock: async (owner, password) => {
          if (!(await verify(owner, password))) return false
          update(owner, { locked: false })
          return true
        },
        // 重新通过登录认证后允许恢复操作；刷新页面不会调用此方法。
        authenticatedLogin: (owner) => {
          if (get().accounts[owner]) update(owner, { locked: false, failedAttempts: 0, retryAt: 0 })
        },
      }
    },
    {
      name: STORE_KEYS.screenLock,
      partialize: (state) => ({ accounts: state.accounts }),
    },
  ),
)

// 同账号的其他标签页同步锁定、解锁及密码变更。
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (event) => {
    if (event.key === STORE_KEYS.screenLock) void useScreenLockStore.persist.rehydrate()
  })
}
