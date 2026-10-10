import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { STORE_KEYS } from '@/constants'

/** 一个打开的页签（标题/图标来自路由 meta，path 唯一） */
export interface TabItem {
  path: string
  title: string
  icon?: string
  /** 固定页签不可关闭（工作台 / 当前身份首页） */
  affix: boolean
}

interface TabsState {
  tabs: TabItem[]
  /** path -> 刷新版本号，变化触发对应页面 remount */
  refreshTicks: Record<string, number>
  /** 路由变化时打开/更新页签（已存在则刷新 affix/标题，不改变顺序） */
  openTab: (tab: TabItem) => void
  /** 关闭单个页签（固定页签忽略），返回剩余页签供调用方决定跳转 */
  closeTab: (path: string) => TabItem[]
  closeOthers: (path: string) => void
  closeLeft: (path: string) => void
  closeRight: (path: string) => void
  /** 关闭全部非固定页签，返回剩余页签 */
  closeAll: () => TabItem[]
  /** 刷新当前页：版本号 +1，KeepAliveOutlet 据此 remount */
  refresh: (path: string) => void
  /** 按当前权限/菜单裁剪本地恢复的页签 */
  pruneByPaths: (allowedPaths: string[]) => void
  /** 退出登录 / 切换账号时清空 */
  resetTabs: () => void
}

export const useTabsStore = create<TabsState>()(
  persist(
    (set, get) => ({
      tabs: [],
      refreshTicks: {},

      openTab: (tab) =>
        set((state) => {
          const index = state.tabs.findIndex((item) => item.path === tab.path)
          if (index === -1) return { tabs: [...state.tabs, tab] }
          const next = state.tabs.slice()
          next[index] = { ...next[index], ...tab }
          return { tabs: next }
        }),

      closeTab: (path) => {
        const remaining = get().tabs.filter((tab) => tab.affix || tab.path !== path)
        set({ tabs: remaining })
        return remaining
      },

      closeOthers: (path) =>
        set((state) => ({
          tabs: state.tabs.filter((tab) => tab.affix || tab.path === path),
        })),

      closeLeft: (path) =>
        set((state) => ({
          tabs: state.tabs.filter((tab, index) => {
            const currentIndex = state.tabs.findIndex((item) => item.path === path)
            return tab.affix || index >= currentIndex
          }),
        })),

      closeRight: (path) =>
        set((state) => ({
          tabs: state.tabs.filter((tab, index) => {
            const currentIndex = state.tabs.findIndex((item) => item.path === path)
            return tab.affix || index <= currentIndex
          }),
        })),

      closeAll: () => {
        const remaining = get().tabs.filter((tab) => tab.affix)
        set({ tabs: remaining })
        return remaining
      },

      refresh: (path) =>
        set((state) => ({
          refreshTicks: { ...state.refreshTicks, [path]: (state.refreshTicks[path] ?? 0) + 1 },
        })),

      pruneByPaths: (allowedPaths) =>
        set((state) => ({
          tabs: state.tabs.filter((tab) => allowedPaths.includes(tab.path)),
        })),

      resetTabs: () => set({ tabs: [], refreshTicks: {} }),
    }),
    {
      name: STORE_KEYS.tabs,
      partialize: (state) => ({ tabs: state.tabs }),
    },
  ),
)
