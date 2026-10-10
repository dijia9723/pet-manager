import { useRef, useState } from 'react'
import { PageContainer } from '@/components/PageContainer'
import { ProFormSelect, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import { PlusOutlined } from '@ant-design/icons'
import { Popconfirm, Tag } from 'antd'
import { PermButton } from '@/components/PermButton'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { useMessage } from '@/hooks/useMessage'
import type { ConfigItem, ConfigPayload, ConfigQuery, ConfigValueType, PageResult } from '@/types'

interface ConfigFormValues {
  config_key: string
  name: string
  value: string
  group_key: string
  type: ConfigValueType
  remark: string
  status: string
}

/** 参数配置 API 形态，平台端 /platform/configs 与租户端 /tenant/configs 均满足 */
export interface ConfigApiLike {
  list: (params: ConfigQuery) => Promise<PageResult<ConfigItem>>
  create: (payload: ConfigPayload) => Promise<ConfigItem>
  update: (id: number, payload: ConfigPayload) => Promise<ConfigItem>
  remove: (id: number) => Promise<unknown>
}

interface ConfigManagerProps {
  /** 数据源 API（平台端或租户端） */
  api: ConfigApiLike
  /** 权限点前缀，如 platform.param / tenant.param */
  permPrefix: string
}

const typeValueEnum = {
  string: { text: 'string' },
  number: { text: 'number' },
  bool: { text: 'bool' },
  json: { text: 'json' },
}

const statusValueEnum = {
  enabled: { text: '启用' },
  disabled: { text: '停用' },
}

/**
 * 参数配置管理界面，平台端（sys_config）与租户端（t_config）共用：
 * 仅数据源（api）与权限点前缀（permPrefix）不同，交互、列、抽屉表单保持一致。
 */
export function ConfigManager({ api, permPrefix }: ConfigManagerProps) {
  const actionRef = useRef<ActionType>(null)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ConfigItem | null>(null)
  const message = useMessage()

  const columns: ProColumns<ConfigItem>[] = [
    {
      title: '关键字',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '参数键 / 名称' },
    },
    { title: '参数键', dataIndex: 'config_key', width: 220, ellipsis: true },
    { title: '名称', dataIndex: 'name', width: 160 },
    {
      title: '值',
      dataIndex: 'value',
      search: false,
      ellipsis: true,
      render: (_, r) => r.value || '—',
    },
    { title: '分组', dataIndex: 'group_key', width: 120, render: (_, r) => r.group_key || '—' },
    { title: '类型', dataIndex: 'type', width: 100, valueType: 'select', valueEnum: typeValueEnum },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      valueType: 'select',
      valueEnum: statusValueEnum,
      render: (_, r) => (r.status === 'enabled' ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
    },
    { title: '备注', dataIndex: 'remark', search: false, ellipsis: true },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      valueType: 'dateTime',
      search: false,
      width: 170,
    },
    {
      title: '操作',
      valueType: 'option',
      key: 'option',
      width: 140,
      render: (_, record) => [
        <PermButton
          key="edit"
          perm={`${permPrefix}.update`}
          type="link"
          size="small"
          onClick={() => {
            setEditing(record)
            setOpen(true)
          }}
        >
          编辑
        </PermButton>,
        <Popconfirm
          key="delete"
          title="确认删除该参数？"
          onConfirm={async () => {
            await api.remove(record.id)
            message.success('已删除')
            actionRef.current?.reload()
          }}
        >
          <PermButton perm={`${permPrefix}.delete`} type="link" size="small" danger>
            删除
          </PermButton>
        </Popconfirm>,
      ],
    },
  ]

  return (
    <PageContainer title="参数配置">
      <DataTable<ConfigItem>
        headerTitle="系统参数"
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1200 }}
        request={async (params) => {
          const { current, pageSize, ...rest } = params
          const res = await api.list({ page: current, page_size: pageSize, ...rest })
          return { data: res.list ?? [], total: res.total ?? 0, success: true }
        }}
        toolBarRender={() => [
          <PermButton
            key="create"
            perm={`${permPrefix}.create`}
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null)
              setOpen(true)
            }}
          >
            新建参数
          </PermButton>,
        ]}
      />
      <DataDrawer<ConfigFormValues>
        key={editing?.id ?? 'create'}
        title={editing ? `编辑参数 - ${editing.name}` : '新建参数'}
        open={open}
        onOpenChange={setOpen}
        initialValues={
          editing ?? {
            type: 'string',
            group_key: '',
            status: 'enabled',
          }
        }
        onFinish={async (values) => {
          if (editing) {
            await api.update(editing.id, values)
            message.success('保存成功')
          } else {
            await api.create(values)
            message.success('创建成功')
          }
          actionRef.current?.reload()
          return true
        }}
      >
        <ProFormText
          name="config_key"
          label="参数键"
          rules={[{ required: true, message: '请输入参数键' }]}
          placeholder="如 site.name"
        />
        <ProFormText name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]} />
        <ProFormTextArea name="value" label="值" fieldProps={{ rows: 3 }} />
        <ProFormText name="group_key" label="分组" placeholder="如 basic / security，可留空" />
        <ProFormSelect
          name="type"
          label="类型"
          rules={[{ required: true }]}
          options={Object.entries(typeValueEnum).map(([value, v]) => ({ label: v.text, value }))}
        />
        <ProFormSelect
          name="status"
          label="状态"
          rules={[{ required: true }]}
          options={Object.entries(statusValueEnum).map(([value, v]) => ({ label: v.text, value }))}
        />
        <ProFormTextArea name="remark" label="备注" fieldProps={{ rows: 2 }} />
      </DataDrawer>
    </PageContainer>
  )
}
