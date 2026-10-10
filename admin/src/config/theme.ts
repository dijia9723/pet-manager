import { theme, type ThemeConfig } from 'antd'
import type { ThemeMode, ThemePreset } from '@/store/preferences'

/** 萌宠风格品牌色（与登录页 login.css 蜜桃色板保持一致） */
export const CUTE_PRIMARY = '#f2814f'

interface ThemeOptions {
  preset: ThemePreset
  mode: ThemeMode
  colorPrimary: string
}

/** 风格决定背景与形态，主色始终取用户偏好。 */
export function effectivePrimary({ colorPrimary }: ThemeOptions): string {
  return colorPrimary
}

/* ------------------------------ 经典风格 ------------------------------ */
function buildClassicTheme({ mode, colorPrimary }: ThemeOptions): ThemeConfig {
  return {
    cssVar: true,
    algorithm: mode === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: {
      colorPrimary,
      borderRadius: 8,
      fontSize: 14,
      wireframe: false,
    },
    components: {
      Menu: {
        itemBorderRadius: 8,
        itemMarginInline: 8,
      },
      Tabs: {
        cardPaddingSM: '6px 12px',
      },
      // 表格统一中等密度：单元格纵向留白略收敛于默认 large
      Table: {
        cellPaddingBlock: 12,
        cellPaddingInline: 16,
        headerBorderRadius: 8,
      },
    },
  }
}

/* ------------------------------ 萌宠风格（亮色） ------------------------------ */
const cuteLightComponents: NonNullable<ThemeConfig['components']> = {
  Layout: {
    headerBg: '#fffaf4',
    siderBg: '#fffaf4',
    bodyBg: '#fdf3e9',
  },
  Menu: {
    itemBorderRadius: 12,
    itemMarginInline: 8,
    itemSelectedBg: '#fde4d2',
    itemSelectedColor: '#d96a3d',
    itemHoverBg: '#fdf0e4',
    itemColor: '#7d6249',
  },
  Table: {
    cellPaddingBlock: 12,
    cellPaddingInline: 16,
    headerBorderRadius: 12,
    headerBg: '#fbeadd',
    headerColor: '#8a5f43',
    rowHoverBg: '#fef5ec',
    borderColor: '#f6e9dc',
  },
  Card: {
    borderRadiusLG: 16,
  },
  Button: {
    borderRadius: 10,
    primaryShadow: '0 8px 18px -8px rgba(242,129,79,0.55)',
  },
  Input: {
    borderRadius: 10,
    hoverBorderColor: '#ffc9a1',
    activeBorderColor: CUTE_PRIMARY,
    activeShadow: '0 0 0 3px rgba(248,160,110,0.18)',
  },
  Select: {
    borderRadius: 10,
  },
  Tabs: {
    itemColor: '#9a7f66',
    itemHoverColor: CUTE_PRIMARY,
    itemSelectedColor: CUTE_PRIMARY,
    inkBarColor: CUTE_PRIMARY,
  },
  Segmented: {
    itemSelectedBg: '#ffffff',
    itemHoverBg: '#fdf0e4',
    trackPadding: 4,
  },
  Tooltip: {
    colorBgSpotlight: '#6b4f3a',
  },
  Tag: {
    borderRadiusSM: 6,
  },
  Modal: {
    borderRadiusLG: 16,
  },
}

/* ------------------------------ 萌宠风格（暗色） ------------------------------ */
const cuteDarkComponents: NonNullable<ThemeConfig['components']> = {
  ...cuteLightComponents,
  Layout: {
    headerBg: '#261d17',
    siderBg: '#261d17',
    bodyBg: '#201813',
  },
  Menu: {
    ...cuteLightComponents.Menu,
    darkItemBg: 'transparent',
    darkSubMenuItemBg: 'transparent',
    darkItemColor: '#c8b09d',
    darkItemHoverColor: '#ffd9bd',
    darkItemSelectedBg: 'rgba(242,129,79,0.22)',
    darkItemSelectedColor: '#ffb98c',
  },
  Table: {
    cellPaddingBlock: 12,
    cellPaddingInline: 16,
    headerBorderRadius: 12,
    headerBg: '#332720',
    headerColor: '#d9bfa9',
    rowHoverBg: 'rgba(242,129,79,0.12)',
    borderColor: '#3d2f25',
  },
  Card: {
    borderRadiusLG: 16,
  },
  Button: {
    borderRadius: 10,
  },
  Input: {
    borderRadius: 10,
    activeShadow: '0 0 0 3px rgba(242,129,79,0.22)',
  },
  Tabs: {
    itemColor: '#c8b09d',
    itemHoverColor: '#ffb98c',
    itemSelectedColor: '#ffb98c',
    inkBarColor: CUTE_PRIMARY,
  },
  Segmented: {
    itemSelectedBg: '#45362c',
    itemHoverBg: '#3d2f25',
    trackPadding: 4,
  },
  Tooltip: {
    colorBgSpotlight: '#5a4131',
  },
}

function buildCuteTheme({ mode, colorPrimary }: ThemeOptions): ThemeConfig {
  const isDark = mode === 'dark'
  const palette = theme.getDesignToken({
    algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: { colorPrimary },
  })
  const baseComponents = isDark ? cuteDarkComponents : cuteLightComponents
  return {
    cssVar: true,
    algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    token: isDark
      ? {
          // 暗色暖棕：蜜桃主色 + 暖向中性色
          colorPrimary,
          colorError: '#f47d6e',
          colorWarning: '#f7b955',
          colorSuccess: '#7bc087',
          borderRadius: 12,
          fontSize: 14,
          wireframe: false,
          colorBgLayout: '#201813',
          colorBgContainer: '#2b211b',
          colorBgElevated: '#342820',
          colorText: '#f3e7dc',
          colorTextSecondary: '#c8b09d',
          colorTextTertiary: '#a48a76',
          colorTextQuaternary: '#7c6655',
          colorBorder: '#4a3a2e',
          colorBorderSecondary: '#392c23',
          colorFill: 'rgba(255,255,255,0.08)',
          colorFillSecondary: 'rgba(255,255,255,0.06)',
          colorFillTertiary: 'rgba(255,255,255,0.04)',
          colorFillQuaternary: 'rgba(255,255,255,0.02)',
          boxShadow: '0 12px 32px -14px rgba(0,0,0,0.55)',
          boxShadowSecondary: '0 8px 24px -12px rgba(0,0,0,0.5)',
        }
      : {
          // 亮色奶油：与登录页同色系
          colorPrimary,
          colorError: '#f47d6e',
          colorWarning: '#f7b955',
          colorSuccess: '#6fbf73',
          borderRadius: 12,
          fontSize: 14,
          wireframe: false,
          colorBgLayout: '#fdf3e9',
          colorBgContainer: '#ffffff',
          colorBgElevated: '#ffffff',
          colorText: '#5a4432',
          colorTextSecondary: '#9a7f66',
          colorTextTertiary: '#b79e86',
          colorTextQuaternary: '#d3bfab',
          colorBorder: '#f0ddc9',
          colorBorderSecondary: '#f6e9dc',
          colorFill: '#f5e7d9',
          colorFillSecondary: '#f8efe4',
          colorFillTertiary: '#fbf4eb',
          colorFillQuaternary: '#fdf9f4',
          boxShadow: '0 16px 40px -18px rgba(191,121,78,0.32)',
          boxShadowSecondary: '0 10px 28px -14px rgba(191,121,78,0.28)',
        },
    components: {
      ...baseComponents,
      Menu: {
        ...baseComponents.Menu,
        itemSelectedBg: palette.colorPrimaryBg,
        itemSelectedColor: palette.colorPrimary,
        itemHoverBg: palette.colorPrimaryBgHover,
        darkItemSelectedBg: palette.colorPrimaryBg,
        darkItemSelectedColor: palette.colorPrimary,
        darkItemHoverColor: palette.colorPrimaryHover,
      },
      Button: { borderRadius: 10 },
      Input: {
        borderRadius: 10,
        hoverBorderColor: palette.colorPrimaryHover,
        activeBorderColor: palette.colorPrimary,
        activeShadow: `0 0 0 3px ${palette.colorPrimaryBg}`,
      },
      Tabs: {
        ...baseComponents.Tabs,
        itemHoverColor: palette.colorPrimaryHover,
        itemSelectedColor: palette.colorPrimary,
        inkBarColor: palette.colorPrimary,
      },
    },
  }
}

/** 按偏好生成 ConfigProvider 主题，两种风格均支持自定义主色与亮暗模式。 */
export function buildAppTheme(options: ThemeOptions): ThemeConfig {
  return options.preset === 'cute' ? buildCuteTheme(options) : buildClassicTheme(options)
}
