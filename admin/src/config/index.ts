/**
 * 应用配置：环境变量的唯一类型化出口。
 * 业务代码一律从这里读取配置，不要直接访问 import.meta.env。
 */
const env = import.meta.env

export const appConfig = {
  /** 应用标题（登录页 / 浏览器标题后缀） */
  title: env.VITE_APP_TITLE || '宠物商户 SaaS 管理后台',
  /** 命名空间：本地缓存键前缀 */
  namespace: env.VITE_APP_NAMESPACE || 'pet-manager-admin',
  /** 后端接口前缀（axios baseURL） */
  baseApi: env.VITE_BASE_API || '/api/v1',
  /** dev 代理目标后端（仅 vite.config 使用） */
  proxyTarget: env.VITE_PROXY_TARGET || 'http://127.0.0.1:8088',
} as const
