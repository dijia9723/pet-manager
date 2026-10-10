import { useRef, useState } from 'react'
import { PageContainer } from '@/components/PageContainer'
import {
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components'
import { PlusOutlined } from '@ant-design/icons'
import { Drawer, Popconfirm, Tag } from 'antd'
import { PermButton } from '@/components/PermButton'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { useMessage } from '@/hooks/useMessage'
import { formatBytes, GiB } from '@/api/storage'
import type {
  DictEntry,
  DictEntryPayload,
  DictType,
  DictTypePayload,
  DictTypeQuery,
  PageResult,
} from '@/types'

interface DictFormValues {
  code: string
  name: string
  remark: string
  status: string
}

interface EntryFormValues {
  storage_gib?: number
  label: string
  value: string
  sort: number
  status: string
  remark: string
}

/** 字典管理 API 形态，平台端 /platform/dicts 与租户端 /tenant/dicts 均满足 */
export interface DictApiLike {
  listTypes: (params: DictTypeQuery) => Promise<PageResult<DictType>>
  createType: (payload: DictTypePayload) => Promise<DictType>
  updateType: (id: number, payload: DictTypePayload) => Promise<DictType>
  removeType: (id: number) => Promise<unknown>
  listEntries: (dictId: number) => Promise<DictEntry[]>
  createEntry: (dictId: number, payload: DictEntryPayload) => Promise<DictEntry>
  updateEntry: (dictId: number, itemId: number, payload: DictEntryPayload) => Promise<DictEntry>
  removeEntry: (dictId: number, itemId: number) => Promise<unknown>
}

interface DictManagerProps {
  /** 数据源 API（平台端或租户端） */
  api: DictApiLike
  /** 权限点前缀，如 platform.dict / tenant.dict */
  permPrefix: string
}

const statusValueEnum = {
  enabled: { text: '启用' },
  disabled: { text: '停用' },
}

/**
 * 字典管理界面（左侧字典类型，抽屉内管理字典项），平台端与租户端共用：
 * 仅数据源（api）与权限点前缀（permPrefix）不同。
 */
export function DictManager({ api, permPrefix }: DictManagerProps) {
  const actionRef = useRef<ActionType>(null)
  const message = useMessage()

  // 字典类型抽屉
  const [dictOpen, setDictOpen] = useState(false)
  const [editingDict, setEditingDict] = useState<DictType | null>(null)

  // 字典项抽屉
  const [activeDict, setActiveDict] = useState<DictType | null>(null)
  const [entries, setEntries] = useState<DictEntry[]>([])
  const [entriesLoading, setEntriesLoading] = useState(false)
  const [entryOpen, setEntryOpen] = useState(false)
  const [editingEntry, setEditingEntry] = useState<DictEntry | null>(null)
  const isStoragePlan = permPrefix === 'platform.dict' && activeDict?.code === 'storage.plan'

  const reloadEntries = async (dictId: number) => {
    setEntriesLoading(true)
    try {
      const data = await api.listEntries(dictId)
      setEntries(data ?? [])
    } finally {
      setEntriesLoading(false)
    }
  }

  const dictColumns: ProColumns<DictType>[] = [
    {
      title: '关键字',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '编码 / 名称' },
    },
    { title: '编码', dataIndex: 'code', width: 200, ellipsis: true },
    { title: '名称', dataIndex: 'name', width: 160 },
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
      width: 200,
      render: (_, record) => [
        <PermButton
          key="entries"
          perm={`${permPrefix}.list`}
          type="link"
          size="small"
          onClick={() => {
            setActiveDict(record)
            void reloadEntries(record.id)
          }}
        >
          字典项
        </PermButton>,
        <PermButton
          key="edit"
          perm={`${permPrefix}.update`}
          type="link"
          size="small"
          onClick={() => {
            setEditingDict(record)
            setDictOpen(true)
          }}
        >
          编辑
        </PermButton>,
        <Popconfirm
          key="delete"
          title="确认删除该字典？"
          description="其下所有字典项会一并删除。"
          onConfirm={async () => {
            await api.removeType(record.id)
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

  const entryColumns: ProColumns<DictEntry>[] = [
    { title: '标签', dataIndex: 'label', width: 160 },
    { title: '值', dataIndex: 'value', width: 160 },
    { title: '排序', dataIndex: 'sort', width: 80 },
    {
      title: '存储容量',
      dataIndex: 'storage_bytes',
      hideInTable: !isStoragePlan,
      width: 120,
      renderText: (v) => formatBytes(v ?? 0),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (_, r) => (r.status === 'enabled' ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
    },
    { title: '备注', dataIndex: 'remark', ellipsis: true },
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
            setEditingEntry(record)
            setEntryOpen(true)
          }}
        >
          编辑
        </PermButton>,
        <Popconfirm
          key="delete"
          title="确认删除该字典项？"
          onConfirm={async () => {
            if (!activeDict) return
            await api.removeEntry(activeDict.id, record.id)
            message.success('已删除')
            void reloadEntries(activeDict.id)
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
    <PageContainer title="字典管理">
      <DataTable<DictType>
        headerTitle="字典类型"
        rowKey="id"
        actionRef={actionRef}
        columns={dictColumns}
        scroll={{ x: 1100 }}
        request={async (params) => {
          const { current, pageSize, ...rest } = params
          const res = await api.listTypes({ page: current, page_size: pageSize, ...rest })
          return { data: res.list ?? [], total: res.total ?? 0, success: true }
        }}
        toolBarRender={() => [
          <PermButton
            key="create"
            perm={`${permPrefix}.create`}
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditingDict(null)
              setDictOpen(true)
            }}
          >
            新建字典
          </PermButton>,
        ]}
      />

      <DataDrawer<DictFormValues>
        key={editingDict?.id ?? 'create'}
        title={editingDict ? `编辑字典 - ${editingDict.name}` : '新建字典'}
        open={dictOpen}
        onOpenChange={setDictOpen}
        initialValues={editingDict ?? { status: 'enabled' }}
        onFinish={async (values) => {
          if (editingDict) {
            await api.updateType(editingDict.id, values)
            message.success('保存成功')
          } else {
            await api.createType(values)
            message.success('创建成功')
          }
          actionRef.current?.reload()
          return true
        }}
      >
        <ProFormText
          name="code"
          label="字典编码"
          rules={[{ required: true, message: '请输入字典编码' }]}
          placeholder="如 pet.gender"
        />
        <ProFormText name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]} />
        <ProFormSelect
          name="status"
          label="状态"
          rules={[{ required: true }]}
          options={Object.entries(statusValueEnum).map(([value, v]) => ({ label: v.text, value }))}
        />
        <ProFormTextArea name="remark" label="备注" fieldProps={{ rows: 2 }} />
      </DataDrawer>

      <Drawer
        title={activeDict ? `字典项 - ${activeDict.name}` : '字典项'}
        width={760}
        open={!!activeDict}
        onClose={() => setActiveDict(null)}
        destroyOnHidden
      >
        <DataTable<DictEntry>
          rowKey="id"
          columns={entryColumns}
          dataSource={entries}
          loading={entriesLoading}
          search={false}
          options={false}
          pagination={false}
          toolBarRender={() => [
            <PermButton
              key="create"
              perm={`${permPrefix}.create`}
              type="primary"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => {
                setEditingEntry(null)
                setEntryOpen(true)
              }}
            >
              新建字典项
            </PermButton>,
          ]}
        />

        <DataDrawer<EntryFormValues>
          width={480}
          key={editingEntry?.id ?? 'create'}
          title={editingEntry ? `编辑字典项 - ${editingEntry.label}` : '新建字典项'}
          open={entryOpen}
          onOpenChange={setEntryOpen}
          initialValues={
            editingEntry
              ? { ...editingEntry, storage_gib: (editingEntry.storage_bytes ?? 0) / GiB }
              : { sort: 0, status: 'enabled', storage_gib: 1 }
          }
          onFinish={async (values) => {
            if (!activeDict) return false
            const { storage_gib, ...rest } = values
            const payload = {
              ...rest,
              storage_bytes: isStoragePlan
                ? Math.round((storage_gib ?? 0) * GiB)
                : (editingEntry?.storage_bytes ?? 0),
            }
            if (editingEntry) {
              await api.updateEntry(activeDict.id, editingEntry.id, payload)
              message.success('保存成功')
            } else {
              await api.createEntry(activeDict.id, payload)
              message.success('创建成功')
            }
            void reloadEntries(activeDict.id)
            return true
          }}
        >
          <ProFormText
            name="label"
            label="标签"
            rules={[{ required: true, message: '请输入标签' }]}
          />
          <ProFormText name="value" label="值" rules={[{ required: true, message: '请输入值' }]} />
          {isStoragePlan && (
            <ProFormDigit
              name="storage_gib"
              label="套餐容量（GiB）"
              min={0.01}
              max={1048576}
              fieldProps={{ precision: 2 }}
              rules={[{ required: true, message: '请输入套餐容量' }]}
              extra="排序最小的启用套餐作为默认等级。已分配租户需在存储资源页重新分配套餐才更新容量。"
            />
          )}
          <ProFormDigit
            name="sort"
            label="排序"
            min={0}
            rules={[{ required: true, message: '请输入排序值' }]}
          />
          <ProFormSelect
            name="status"
            label="状态"
            rules={[{ required: true }]}
            options={Object.entries(statusValueEnum).map(([value, v]) => ({
              label: v.text,
              value,
            }))}
          />
          <ProFormTextArea name="remark" label="备注" fieldProps={{ rows: 2 }} />
        </DataDrawer>
      </Drawer>
    </PageContainer>
  )
}
