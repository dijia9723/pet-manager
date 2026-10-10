import { DrawerForm, type DrawerFormProps } from '@ant-design/pro-components'

/**
 * 业务表单抽屉二次封装：固化项目级约定。
 *
 * 统一约定：
 * - 默认宽度 520（角色授权 / 菜单配置等宽场景传 560，轻量表单可传 420）
 * - 关闭即销毁表单实例，避免多实体表单串值（drawerProps.destroyOnHidden）
 * - 调用方仍可用 drawerProps 追加任意 Drawer 配置
 */
export type DataDrawerProps<T = Record<string, any>> = DrawerFormProps<T>

export function DataDrawer<T = Record<string, any>>({
  width = 520,
  drawerProps,
  ...rest
}: DataDrawerProps<T>) {
  return (
    <DrawerForm<T>
      width={width}
      drawerProps={{ destroyOnHidden: true, ...drawerProps }}
      {...rest}
    />
  )
}
