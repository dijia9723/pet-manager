import type { ReactNode } from 'react'
import {
  AppstoreOutlined,
  ClockCircleOutlined,
  ControlOutlined,
  DashboardOutlined,
  FileTextOutlined,
  MenuOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  ShopOutlined,
  TeamOutlined,
  UserOutlined,
  BellOutlined,
  CloudOutlined,
  HddOutlined,
  GlobalOutlined,
} from '@ant-design/icons'
import type { MenuDataItem } from '@ant-design/pro-components'
import type { MenuNode } from '@/types'

/** 图标标识 → 组件；后端返回的 icon 名称优先按 key 匹配，未命中给默认图标 */
const iconMap: Record<string, ReactNode> = {
  server: <HddOutlined />,
  website: <GlobalOutlined />,
  storage: <CloudOutlined />,
  notification: <BellOutlined />,
  dashboard: <DashboardOutlined />,
  shop: <ShopOutlined />,
  clock: <ClockCircleOutlined />,
  team: <TeamOutlined />,
  safety: <SafetyCertificateOutlined />,
  appstore: <AppstoreOutlined />,
  setting: <SettingOutlined />,
  user: <UserOutlined />,
  log: <FileTextOutlined />,
  config: <ControlOutlined />,
}

export function renderIcon(name?: string): ReactNode {
  if (!name) return undefined
  return iconMap[name] ?? <MenuOutlined />
}

/** 后端菜单树 → ProLayout 菜单数据 */
export function toMenuData(nodes: MenuNode[]): MenuDataItem[] {
  return (
    nodes
      .map((node) => ({
        key: node.key,
        path: node.path,
        name: node.title,
        icon: renderIcon(node.icon),
        children: node.children?.length ? toMenuData(node.children) : undefined,
      }))
      // 空 path 且无子节点的父级是纯容器菜单，剪掉避免渲染成空白项（与后端 pruneEmptyMenus 对齐）
      .filter((item) => item.path !== '' || (item.children?.length ?? 0) > 0)
  )
}
