import type { ComponentType, LazyExoticComponent } from 'react'

/** 业务页面组件（统一用 React.lazy 包裹，构建时自动 code-splitting） */
export type PageComponent = ComponentType

/**
 * 业务路由模块（Vben mixed 模式的 React 实现）：
 * 本地声明路由 + meta 用于路由落地 / 菜单兜底 / 页签 / 缓存，
 * 菜单展示优先用后端 /auth/menus（已按权限过滤）。
 */
export interface AppRouteModule {
  /** 唯一标识 */
  key: string
  /** 绝对路径，如 /platform/tenants */
  path: string
  /** 菜单名 / 页面标题 / 页签标题 */
  title: string
  /** 图标标识，见 layouts/menu.tsx 的 iconMap，也兼容后端 icon 名称 */
  icon?: string
  /** 需要的单个权限点（缺省仅需登录） */
  perm?: string
  /** 满足任意权限点即可 */
  anyOf?: string[]
  /** 菜单排序，越小越靠前（本地兜底菜单使用） */
  order?: number
  /** 不在菜单中显示（仍可路由访问） */
  hideInMenu?: boolean
  /** 固定页签（不可关闭）；此外当前用户首页会动态固定 */
  affixTab?: boolean
  /**
   * 是否缓存页面实例（多标签页切走后保留状态），默认 true。
   * 注意：React 无原生 keep-alive，缓存实现为 display:none；
   * 带 :id 参数的详情页等直接依赖路由 location 的页面建议设为 false。
   */
  keepAlive?: boolean
  /** 懒加载页面组件 */
  component: LazyExoticComponent<PageComponent>
}
