import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { STORE_KEYS } from '@/constants'

/** 布局模式，对齐 antd ProLayout 的 side / top / mix */
export type LayoutMode = 'side' | 'mix' | 'top'
export type ThemeMode = 'light' | 'dark'
/** 主题风格：经典商务蓝 / 萌宠奶油蜜桃 */
export type ThemePreset = 'classic' | 'cute'

interface PreferencesState {
  /** 亮色 / 暗色 */
  themeMode: ThemeMode
  /** 主题风格 */
  themePreset: ThemePreset
  /** 主题色（两种风格均可自定义） */
  colorPrimary: string
  /** 布局模式 */
  layout: LayoutMode
  /** 多标签页开关 */
  tabbarEnable: boolean
  /** 页面实例缓存总开关（路由 meta.keepAlive 可逐页关闭） */
  keepAliveEnable: boolean
  /** 面包屑开关 */
  breadcrumbEnable: boolean
  /** 水印开关 */
  watermarkEnable: boolean
  watermarkContent: string
  watermarkOpacity: number
  /** 侧边栏折叠（持久化用户习惯） */
  collapsed: boolean
  /** 局部更新偏好 */
  setPref: (patch: Partial<Omit<PreferencesState, 'setPref' | 'toggleTheme' | 'reset'>>) => void
  toggleTheme: () => void
  reset: () => void
}

/** 框架默认偏好（对标 Vben defaultPreferences 的常用子集） */
export const defaultPreferences = {
  themeMode: 'light' as ThemeMode,
  themePreset: 'classic' as ThemePreset,
  colorPrimary: '#1677ff',
  layout: 'mix' as LayoutMode,
  tabbarEnable: true,
  keepAliveEnable: true,
  breadcrumbEnable: true,
  watermarkEnable: false,
  watermarkContent: '',
  watermarkOpacity: 16,
  collapsed: false,
}

export const usePreferencesStore = create<PreferencesState>()(
  persist(
    (set) => ({
      ...defaultPreferences,

      setPref: (patch) => set(patch),

      toggleTheme: () =>
        set((state) => ({ themeMode: state.themeMode === 'dark' ? 'light' : 'dark' })),

      reset: () => set({ ...defaultPreferences }),
    }),
    {
      name: STORE_KEYS.preferences,
      version: 1,
      migrate: (persisted) => {
        const previous = persisted as Partial<PreferencesState>
        return {
          ...defaultPreferences,
          ...previous,
          // 旧版萌宠风格实际使用固定蜜桃色，迁移时保留原来的视觉效果。
          colorPrimary:
            previous.themePreset === 'cute' ? '#f2814f' : previous.colorPrimary || '#1677ff',
        }
      },
      partialize: (state) => ({
        themeMode: state.themeMode,
        themePreset: state.themePreset,
        colorPrimary: state.colorPrimary,
        layout: state.layout,
        tabbarEnable: state.tabbarEnable,
        keepAliveEnable: state.keepAliveEnable,
        breadcrumbEnable: state.breadcrumbEnable,
        watermarkEnable: state.watermarkEnable,
        watermarkContent: state.watermarkContent,
        watermarkOpacity: state.watermarkOpacity,
        collapsed: state.collapsed,
      }),
    },
  ),
)
