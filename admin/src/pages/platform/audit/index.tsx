import { PageContainer } from '@/components/PageContainer'
import { Tag } from 'antd'
import { auditApi } from '@/api/audit'
import { DataTable, type ProColumns } from '@/components/ui'
import type { AuditLogItem, AuditLogQuery } from '@/types'

const methodEnum = {
  POST: { text: 'POST' },
  PUT: { text: 'PUT' },
  PATCH: { text: 'PATCH' },
  DELETE: { text: 'DELETE' },
}

/** 平台端：审计日志（只读，不提供编辑/删除） */
export default function AuditLogPage() {
  const columns: ProColumns<AuditLogItem>[] = [
    { title: '时间', dataIndex: 'created_at', valueType: 'dateTime', search: false, width: 170 },
    { title: '操作人', dataIndex: 'username', width: 120 },
    {
      title: '账号ID',
      dataIndex: 'user_id',
      width: 90,
      search: false,
      render: (_, r) => r.user_id || '-',
    },
    {
      title: '身份',
      dataIndex: 'user_type',
      search: false,
      width: 90,
      render: (_, r) =>
        r.user_type === 'platform' ? <Tag color="blue">平台</Tag> : <Tag>租户</Tag>,
    },
    {
      title: '租户ID',
      dataIndex: 'tenant_id',
      width: 90,
      search: false,
      render: (_, r) => r.tenant_id || '—',
    },
    {
      title: '方法',
      dataIndex: 'method',
      valueType: 'select',
      valueEnum: methodEnum,
      width: 90,
      render: (_, r) => {
        const colorMap: Record<string, string> = {
          POST: 'blue',
          PUT: 'orange',
          PATCH: 'cyan',
          DELETE: 'red',
        }
        return <Tag color={colorMap[r.method] || 'default'}>{r.method}</Tag>
      },
    },
    { title: '请求路径', dataIndex: 'path', ellipsis: true },
    { title: '路由', dataIndex: 'route', search: false, ellipsis: true, hideInTable: true },
    {
      title: '状态码',
      dataIndex: 'status',
      width: 90,
      render: (_, r) => {
        const code = r.status
        const color = code < 300 ? 'green' : code < 500 ? 'orange' : 'red'
        return <Tag color={color}>{code}</Tag>
      },
    },
    { title: '耗时(ms)', dataIndex: 'latency_ms', search: false, width: 100 },
    { title: '来源IP', dataIndex: 'client_ip', search: false, width: 130 },
    { title: 'RequestID', dataIndex: 'request_id', search: false, width: 170, ellipsis: true },
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
    <PageContainer title="审计日志">
      <DataTable<AuditLogItem>
        headerTitle="写操作记录"
        rowKey="id"
        columns={columns}
        scroll={{ x: 1400 }}
        request={async (params) => {
          const { current, pageSize, start, end, ...rest } = params
          const query: AuditLogQuery = {
            page: current,
            page_size: pageSize,
            start,
            end,
            ...rest,
          }
          const res = await auditApi.list(query)
          return { data: res.list ?? [], total: res.total ?? 0, success: true }
        }}
      />
    </PageContainer>
  )
}
