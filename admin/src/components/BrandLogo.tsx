import type { CSSProperties } from 'react'

/**
 * 品牌标识：渐变圆角底 + 白色爪印。
 * 纯内联 SVG，不依赖图片资源；颜色随当前主题色（active/primary/hover 三档渐变）。
 */
interface BrandLogoProps {
  size?: number
  /** 仅渲染爪印（无渐变底），颜色由 currentColor 控制 */
  plain?: boolean
  className?: string
  style?: CSSProperties
}

function Paw({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 64 64" fill="currentColor" aria-hidden="true">
      <ellipse cx="16" cy="26" rx="6.5" ry="8.5" transform="rotate(-18 16 26)" />
      <ellipse cx="27" cy="17" rx="6.5" ry="9" />
      <ellipse cx="38" cy="17" rx="6.5" ry="9" />
      <ellipse cx="49" cy="26" rx="6.5" ry="8.5" transform="rotate(18 49 26)" />
      <path d="M32 27c-9 0-15.5 6.5-15.5 13.5 0 5.5 4 9 9.5 9 2.3 0 3.6-1.1 6-1.1s3.7 1.1 6 1.1c5.5 0 9.5-3.5 9.5-9C47.5 33.5 41 27 32 27z" />
    </svg>
  )
}

export function BrandLogo({ size = 32, plain = false, className, style }: BrandLogoProps) {
  if (plain) {
    return (
      <span className={className} style={{ display: 'inline-flex', color: 'currentColor', ...style }}>
        <Paw size={size} />
      </span>
    )
  }
  return (
    <span
      className={`app-logo${className ? ` ${className}` : ''}`}
      style={{ width: size, height: size, borderRadius: Math.round(size * 0.28), ...style }}
    >
      <Paw size={Math.round(size * 0.62)} />
    </span>
  )
}
