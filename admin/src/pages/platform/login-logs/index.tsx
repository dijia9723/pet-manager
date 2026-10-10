import { PageContainer } from '@/components/PageContainer'
import { Tag } from 'antd'
import { loginLogApi } from '@/api/loginlog'
import { DataTable, type ProColumns } from '@/components/ui'
import type { LoginLogItem, LoginLogQuery } from '@/types'

/** 平台端：登录日志（只读，成功与失败登录都记录） */
export default function LoginLogPage() {
  const columns: ProColumns<LoginLogItem>[] = [
    { title: '时间', dataIndex: 'created_at', valueType: 'dateTime', search: false, width: 170 },
    { title: '账号', dataIndex: 'username', width: 140 },
    {
      title: '身份',
      dataIndex: 'user_type',
      valueType: 'select',
      width: 100,
      valueEnum: { platform: { text: '平台' }, tenant: { text: '租户' } },
      render: (_, r) =>
        r.user_type === 'platform' ? (
          <Tag color="blue">平台</Tag>
        ) : r.user_type === 'tenant' ? (
          <Tag>租户</Tag>
        ) : (
          '—'
        ),
    },
    {
      title: '租户编码',
      dataIndex: 'tenant_code',
      search: false,
      width: 140,
      render: (_, r) => r.tenant_code || '—',
    },
    {
      title: '租户ID',
      dataIndex: 'tenant_id',
      search: false,
      width: 90,
      render: (_, r) => r.tenant_id || '—',
    },
    {
      title: '结果',
      dataIndex: 'status',
      valueType: 'select',
      width: 90,
      valueEnum: { success: { text: '成功' }, fail: { text: '失败' } },
      render: (_, r) =>
        r.status === 'success' ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>,
    },
    { title: '原因', dataIndex: 'message', search: false, ellipsis: true },
    { title: '来源IP', dataIndex: 'client_ip', search: false, width: 130 },
    { title: 'User-Agent', dataIndex: 'user_agent', search: false, width: 220, ellipsis: true },
    {
      title: '时间范围',
      dataIndex: 'created_at',
      key: 'time_range',
      valueType: 'dateTimeRange',
      hideInTable: true,
      search: {
        transform: (value: [string, string]) => ({ start: value?.[0], end: value?.[1] }),
      },
    },
  ]

  return (
    <PageContainer title="登录日志">
      <DataTable<LoginLogItem>
        headerTitle="登录记录"
        rowKey="id"
        columns={columns}
        scroll={{ x: 1400 }}
        request={async (params) => {
          const { current, pageSize, start, end, ...rest } = params
          const query: LoginLogQuery = {
            page: current,
            page_size: pageSize,
            start,
            end,
            ...rest,
          }
          const res = await loginLogApi.list(query)
          return { data: res.list ?? [], total: res.total ?? 0, success: true }
        }}
      />
    </PageContainer>
  )
}
