import type { CSSProperties } from 'react'

interface IconProps {
  className?: string
  style?: CSSProperties
}

/** 爪印 SVG（大掌垫 + 4 个脚趾），fill 跟随 currentColor */
export function PawMark({ className, style }: IconProps) {
  return (
    <svg viewBox="0 0 64 64" fill="currentColor" className={className} style={style} aria-hidden="true">
      <ellipse cx="15.5" cy="25" rx="6.4" ry="7.4" />
      <ellipse cx="26.5" cy="17.5" rx="6.8" ry="7.8" />
      <ellipse cx="37.5" cy="17.5" rx="6.8" ry="7.8" />
      <ellipse cx="48.5" cy="25" rx="6.4" ry="7.4" />
      <path d="M32 29.5c-11.6 0-19.8 9.2-17.6 17.4 1.9 6.9 11.1 8.2 17.6 1.5 6.5 6.7 15.7 5.4 17.6-1.5C51.8 38.7 43.6 29.5 32 29.5Z" />
    </svg>
  )
}

/** 爱心 SVG */
export function HeartMark({ className, style }: IconProps) {
  return (
    <svg viewBox="0 0 64 64" fill="currentColor" className={className} style={style} aria-hidden="true">
      <path d="M32 55C13.5 41.6 7.5 30.8 14.8 21.4 20.3 14.3 29 16.6 32 23c3-6.4 11.7-8.7 17.2-1.6C56.5 30.8 50.5 41.6 32 55Z" />
    </svg>
  )
}

/** 四角星光 SVG */
export function SparkleMark({ className, style }: IconProps) {
  return (
    <svg viewBox="0 0 64 64" fill="currentColor" className={className} style={style} aria-hidden="true">
      <path d="M32 4c2.6 13.8 6.6 17.8 20.4 20.4C38.6 27 34.6 31 32 44.8 29.4 31 25.4 27 11.6 24.4 25.4 21.8 29.4 17.8 32 4Z" />
    </svg>
  )
}

/** 鱼骨 SVG（底部点缀） */
export function FishboneMark({ className, style }: IconProps) {
  return (
    <svg viewBox="0 0 64 64" fill="currentColor" className={className} style={style} aria-hidden="true">
      <path d="M10 32c0-6.3 5.7-11 14.4-11 6.9 0 13 2.8 16.4 7.1L56 21l-4.2 11L56 43l-15.2-7.1C37.4 40.2 31.3 43 24.4 43 15.7 43 10 38.3 10 32Z" />
      <circle cx="20" cy="29.5" r="2.2" fill="#fff" opacity="0.85" />
    </svg>
  )
}
