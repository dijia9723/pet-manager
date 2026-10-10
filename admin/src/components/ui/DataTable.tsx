import { ProTable, type ProTableProps } from '@ant-design/pro-components'

/**
 * 业务表格二次封装：在 ProTable 之上固化项目级约定，调用方只需关心列与数据。
 *
 * 统一约定（均可用 props 覆盖/关闭）：
 * - size="middle"：中等密度
 * - options：关闭密度切换，保留列设置与刷新
 * - search：查询表单标签宽度自适应
 * - pagination：默认每页 10 条并展示 pageSize 切换（传 false 即不分页）
 */
export type DataTableProps<T, U extends Record<string, any> = Record<string, any>> = ProTableProps<
  T,
  U
>

const DEFAULT_OPTIONS = {
  density: false,
  setting: true,
  reload: true,
} as const

const DEFAULT_PAGINATION = {
  defaultPageSize: 10,
  showSizeChanger: true,
} as const

const DEFAULT_SEARCH = {
  labelWidth: 'auto' as const,
}

export function DataTable<
  T extends Record<string, any>,
  U extends Record<string, any> = Record<string, any>,
>(props: DataTableProps<T, U>) {
  const { size, options, pagination, search, ...rest } = props

  return (
    <ProTable<T, U>
      size={size ?? 'middle'}
      options={options === undefined ? DEFAULT_OPTIONS : options}
      pagination={pagination === undefined ? DEFAULT_PAGINATION : pagination}
      search={search === undefined ? DEFAULT_SEARCH : search}
      {...rest}
    />
  )
}
