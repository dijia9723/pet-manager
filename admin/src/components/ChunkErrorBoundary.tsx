import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
}

/**
 * 懒加载 chunk 失败兜底。
 * 典型场景：发版后旧页面引用的 js chunk 文件名（带 hash）已不存在，
 * 浏览器动态 import 抛错；此时自动刷新一次拿到最新 index.html 即可恢复。
 * 模块级 reloading 标记防止刷新死循环。
 */
let chunkReloading = false

function isChunkLoadError(error: unknown): boolean {
  const message = error instanceof Error ? error.message : String(error)
  return /Failed to fetch dynamically imported module|error loading dynamically imported module|Importing a module script failed/i.test(
    message,
  )
}

export class ChunkErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    if (isChunkLoadError(error) && !chunkReloading) {
      chunkReloading = true
      window.location.reload()
      return
    }
    // 非 chunk 错误输出到控制台，避免静默
    console.error('Route render error:', error, info)
  }

  render() {
    if (this.state.hasError) return null
    return this.props.children
  }
}
