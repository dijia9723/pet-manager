import { App } from 'antd'

/**
 * 获取消费 ConfigProvider 上下文的 message 实例（替代 antd 静态 message）。
 * 必须在 <App> 树内使用（main.tsx 已包裹）。
 */
export function useMessage() {
  return App.useApp().message
}
