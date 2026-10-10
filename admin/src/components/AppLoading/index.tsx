import type { CSSProperties, ReactNode } from 'react'
import './style.css'

export interface AppLoadingProps {
  /** 球体直径 px，默认 56 */
  size?: number
  /** 提示文案，传 null 不显示，默认「页面加载中」+ 跳动圆点 */
  tip?: ReactNode
  /** 是否作为整页兜底（垂直居中，min-height 60vh），默认 true */
  page?: boolean
  className?: string
  style?: CSSProperties
}

/**
 * 全站统一加载态：精灵球弹性摇摆 loader。
 * 路由懒加载用 <AppLoading />；局部小区域可用 page={false} 或自定义 size。
 */
export function AppLoading({
  size = 56,
  tip = '页面加载中',
  page = true,
  className,
  style,
}: AppLoadingProps) {
  const classes = ['app-loading', page ? 'is-page' : '', className ?? '']
    .filter(Boolean)
    .join(' ')

  return (
    <div className={classes} style={style} role="status" aria-live="polite">
      <div
        className="app-loading-ball"
        style={{ '--loader-size': `${size}px` } as CSSProperties}
      />
      {tip !== null && tip !== undefined && (
        <div className="app-loading-tip">
          {tip}
          <span className="app-loading-dots">
            <i />
            <i />
            <i />
          </span>
        </div>
      )}
    </div>
  )
}
