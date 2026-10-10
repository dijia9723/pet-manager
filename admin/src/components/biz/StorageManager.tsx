import { useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Drawer,
  Popconfirm,
  Progress,
  Space,
  Tabs,
  Tag,
  Typography,
  Upload,
} from 'antd'
import { UploadOutlined } from '@ant-design/icons'
import { ProFormDigit, ProFormSelect, ProFormTextArea } from '@ant-design/pro-components'
import { Link } from 'react-router-dom'
import { PageContainer } from '@/components/PageContainer'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { PermButton } from '@/components/PermButton'
import { useMessage } from '@/hooks/useMessage'
import { usePermission } from '@/hooks/usePermission'
import { tenantApi } from '@/api/tenant'
import {
  formatBytes,
  GiB,
  openDownload,
  storageApi,
  storagePlatformApi,
  type StorageAccount,
  type StorageChange,
  type StorageResource,
} from '@/api/storage'
import type { UserType } from '@/types'

export function StorageManager({ scope }: { scope: UserType }) {
  return (
    <PageContainer title="存储资源管理">
      {scope === 'platform' ? (
        <Tabs
          destroyOnHidden
          items={[
            { key: 'resources', label: '存储资源', children: <ResourceManager scope={scope} /> },
            { key: 'accounts', label: '租户存储套餐', children: <AccountManager /> },
          ]}
        />
      ) : (
        <ResourceManager scope={scope} />
      )}
    </PageContainer>
  )
}

function ResourceManager({ scope }: { scope: UserType }) {
  const api = storageApi(scope)
  const actionRef = useRef<ActionType>(null)
  const message = useMessage()
  const { has } = usePermission()
  const queryClient = useQueryClient()
  const [uploading, setUploading] = useState(false)
  const { data: summary } = useQuery({ queryKey: ['storage-summary', scope], queryFn: api.summary })
  const reload = () => {
    actionRef.current?.reload()
    void queryClient.invalidateQueries({ queryKey: ['storage-summary', scope] })
  }
  const columns: ProColumns<StorageResource>[] = [
    { title: '文件名', dataIndex: 'keyword', hideInTable: true },
    { title: '文件名', dataIndex: 'name', search: false, ellipsis: true, width: 240 },
    {
      title: '归属',
      dataIndex: 'tenant_name',
      search: false,
      hideInTable: scope === 'tenant',
      width: 160,
    },
    {
      title: '所属空间',
      dataIndex: 'tenant_id',
      hideInTable: true,
      hideInSearch: scope === 'tenant',
      valueType: 'select',
      request: async ({ keyWords }) => {
        const data = await tenantApi.list({ page_size: 200, keyword: keyWords })
        return [
          { label: '平台公共存储', value: 0 },
          ...data.list.map((t) => ({ label: t.name, value: t.id })),
        ]
      },
      fieldProps: { showSearch: true },
    },
    { title: '大小', dataIndex: 'size_bytes', search: false, renderText: formatBytes, width: 110 },
    {
      title: '状态',
      dataIndex: 'status',
      valueEnum: {
        ready: { text: '可用', status: 'Success' },
        uploading: { text: '上传中', status: 'Processing' },
        deleting: { text: '待清理', status: 'Warning' },
      },
      width: 110,
    },
    { title: '上传人', dataIndex: 'uploader_name', search: false, width: 120 },
    {
      title: '上传时间',
      dataIndex: 'created_at',
      valueType: 'dateTime',
      search: false,
      width: 170,
    },
    {
      title: '操作',
      valueType: 'option',
      width: 140,
      fixed: 'right',
      render: (_, r) => [
        r.status === 'ready' && (
          <PermButton
            key="download"
            type="link"
            size="small"
            perm={`${scope}.storage.list`}
            onClick={async () => {
              const data = await api.download(r.id)
              openDownload(data.url)
            }}
          >
            下载
          </PermButton>
        ),
        r.status !== 'uploading' && (
          <Popconfirm
            key="delete"
            title="确认永久删除此文件？"
            description="文件被删除后，引用它的消息附件也将无法下载。"
            onConfirm={async () => {
              try {
                await api.remove(r.id)
                message.success('文件已删除，空间已释放')
              } finally {
                reload()
              }
            }}
          >
            <PermButton type="link" size="small" danger perm={`${scope}.storage.delete`}>
              {r.status === 'deleting' ? '重试清理' : '删除'}
            </PermButton>
          </Popconfirm>
        ),
      ],
    },
  ]
  return (
    <>
      {summary && (
        <Card size="small" style={{ marginBottom: 16 }}>
          <Descriptions
            column={{ xs: 1, sm: 2, lg: 4 }}
            items={[
              { key: 'plan', label: '当前套餐', children: summary.plan_name },
              { key: 'used', label: '已占用', children: formatBytes(summary.used_bytes) },
              {
                key: 'quota',
                label: '总空间',
                children: summary.unlimited ? '平台公共存储' : formatBytes(summary.quota_bytes),
              },
              { key: 'extra', label: '追加扩容', children: formatBytes(summary.extra_bytes) },
            ]}
          />
          {!summary.unlimited && (
            <Progress
              percent={Math.min(
                100,
                Number(((summary.used_bytes / Math.max(1, summary.quota_bytes)) * 100).toFixed(1)),
              )}
            />
          )}
          <Typography.Text type="secondary">
            {scope === 'tenant'
              ? '资源归属本租户，使用平台 TOS。需要更多空间时请联系平台。'
              : '查看并管理平台及各租户的资源；此处上传的文件归属平台公共存储。'}{' '}
            单文件上限 {formatBytes(summary.max_upload_bytes)}。
          </Typography.Text>
        </Card>
      )}
      {summary && !summary.enabled && (
        <Alert
          type="warning"
          showIcon
          message="平台 TOS 存储尚未启用，请检查服务端配置"
          style={{ marginBottom: 16 }}
        />
      )}
      <DataTable<StorageResource>
        rowKey="id"
        headerTitle="文件资源"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1200 }}
        request={async (p) => {
          const data = await api.list({
            page: p.current,
            page_size: p.pageSize,
            keyword: p.keyword,
            status: p.status,
            tenant_id: p.tenant_id,
          })
          return { data: data.list, total: data.total, success: true }
        }}
        toolBarRender={() => [
          has(`${scope}.storage.upload`) && (
            <Upload
              key="upload"
              showUploadList={false}
              disabled={uploading || !summary?.enabled}
              beforeUpload={async (file) => {
                if (!file.size || file.size > (summary?.max_upload_bytes ?? 0)) {
                  message.error(
                    `单文件须大于 0 且不超过 ${formatBytes(summary?.max_upload_bytes ?? 0)}`,
                  )
                  return Upload.LIST_IGNORE
                }
                setUploading(true)
                try {
                  await api.upload(file)
                  message.success('文件已上传')
                  reload()
                } finally {
                  setUploading(false)
                }
                return false
              }}
            >
              <Button
                type="primary"
                icon={<UploadOutlined />}
                loading={uploading}
                disabled={!summary?.enabled}
              >
                上传文件
              </Button>
            </Upload>
          ),
        ]}
      />
    </>
  )
}

function AccountManager() {
  const actionRef = useRef<ActionType>(null)
  const message = useMessage()
  const [editing, setEditing] = useState<{
    account: StorageAccount
    action: 'assign' | 'expand'
  } | null>(null)
  const [history, setHistory] = useState<StorageAccount | null>(null)
  const columns: ProColumns<StorageAccount>[] = [
    { title: '租户名称 / 编码', dataIndex: 'keyword', hideInTable: true },
    { title: '租户名称', dataIndex: 'tenant_name', search: false, width: 160 },
    { title: '租户编码', dataIndex: 'tenant_code', search: false, width: 130 },
    { title: '套餐等级', dataIndex: 'plan_name', search: false },
    { title: '套餐容量', dataIndex: 'base_bytes', search: false, renderText: formatBytes },
    { title: '追加容量', dataIndex: 'extra_bytes', search: false, renderText: formatBytes },
    {
      title: '已占用 / 总空间',
      key: 'usage',
      search: false,
      render: (_, r) => (
        <Space direction="vertical" size={0}>
          <span>
            {formatBytes(r.used_bytes)} / {formatBytes(r.quota_bytes)}
          </span>
          <Progress
            size="small"
            percent={Math.min(100, Math.round((r.used_bytes / Math.max(1, r.quota_bytes)) * 100))}
          />
        </Space>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      width: 230,
      fixed: 'right',
      render: (_, r) => [
        <PermButton
          key="assign"
          type="link"
          size="small"
          perm="platform.storage.allocate"
          onClick={() => setEditing({ account: r, action: 'assign' })}
        >
          分配 / 升级
        </PermButton>,
        <PermButton
          key="expand"
          type="link"
          size="small"
          perm="platform.storage.allocate"
          onClick={() => setEditing({ account: r, action: 'expand' })}
        >
          扩容
        </PermButton>,
        <Button key="history" type="link" size="small" onClick={() => setHistory(r)}>
          记录
        </Button>,
      ],
    },
  ]
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message={
          <span>
            套餐来自平台字典 <Link to="/platform/dicts">storage.plan</Link>
            。字典排序决定默认等级；修改字典后，可重新分配套餐使容量生效。升级保留追加容量。
          </span>
        }
      />
      <DataTable<StorageAccount>
        rowKey="tenant_id"
        headerTitle="租户存储套餐"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1250 }}
        request={async (p) => {
          const data = await storagePlatformApi.accounts({
            page: p.current,
            page_size: p.pageSize,
            keyword: p.keyword,
          })
          return { data: data.list, total: data.total, success: true }
        }}
      />
      {editing && (
        <DataDrawer<{ plan_id: number; add_gib: number; remark: string }>
          title={`${editing.account.tenant_name} · ${editing.action === 'assign' ? '分配 / 升级套餐' : '存储扩容'}`}
          open
          initialValues={{ plan_id: editing.account.plan_id, add_gib: 1 }}
          onOpenChange={(open) => {
            if (!open) setEditing(null)
          }}
          onFinish={async (values) => {
            await storagePlatformApi.allocate(editing.account.tenant_id, {
              action: editing.action,
              plan_id: values.plan_id,
              add_bytes: Math.round((values.add_gib ?? 0) * GiB),
              remark: values.remark,
            })
            message.success('存储配额已更新')
            setEditing(null)
            actionRef.current?.reload()
            return true
          }}
        >
          <Typography.Paragraph>
            当前总空间 {formatBytes(editing.account.quota_bytes)}，已占用{' '}
            {formatBytes(editing.account.used_bytes)}。
          </Typography.Paragraph>
          {editing.action === 'assign' ? (
            <ProFormSelect
              name="plan_id"
              label="存储套餐等级"
              rules={[{ required: true }]}
              request={async () =>
                (await storagePlatformApi.plans()).map((p) => ({
                  value: p.id,
                  label: `${p.label} · ${formatBytes(p.storage_bytes ?? 0)}`,
                }))
              }
            />
          ) : (
            <ProFormDigit
              name="add_gib"
              label="追加容量（GiB）"
              min={0.01}
              max={1048576}
              fieldProps={{ precision: 2 }}
              rules={[{ required: true }]}
            />
          )}
          <ProFormTextArea
            name="remark"
            label="变更备注"
            fieldProps={{ maxLength: 255, rows: 3 }}
          />
        </DataDrawer>
      )}
      <Drawer
        title={`${history?.tenant_name ?? ''} · 配额变更记录`}
        width={900}
        open={!!history}
        onClose={() => setHistory(null)}
        destroyOnHidden
      >
        {history && (
          <DataTable<StorageChange>
            key={history.tenant_id}
            rowKey="id"
            search={false}
            columns={[
              {
                title: '动作',
                dataIndex: 'action',
                render: (_, r) => (
                  <Tag color={r.action === 'expand' ? 'blue' : 'green'}>
                    {r.action === 'expand' ? '扩容' : '分配套餐'}
                  </Tag>
                ),
              },
              { title: '套餐', dataIndex: 'plan_name' },
              { title: '原空间', dataIndex: 'before_bytes', renderText: formatBytes },
              { title: '新空间', dataIndex: 'after_bytes', renderText: formatBytes },
              { title: '操作人', dataIndex: 'operator_name' },
              { title: '备注', dataIndex: 'remark', ellipsis: true },
              { title: '时间', dataIndex: 'created_at', valueType: 'dateTime' },
            ]}
            request={async (p) => {
              const data = await storagePlatformApi.changes(history.tenant_id, {
                page: p.current,
                page_size: p.pageSize,
              })
              return { data: data.list, total: data.total, success: true }
            }}
          />
        )}
      </Drawer>
    </>
  )
}
