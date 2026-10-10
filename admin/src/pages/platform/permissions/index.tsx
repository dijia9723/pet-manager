import { useRef, useState } from 'react'
import { PageContainer } from '@/components/PageContainer'
import {
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTreeSelect,
} from '@ant-design/pro-components'
import { PlusOutlined } from '@ant-design/icons'
import { Popconfirm, Tag } from 'antd'
import { permissionApi } from '@/api/permission'
import { PermButton } from '@/components/PermButton'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { useMessage } from '@/hooks/useMessage'
import type { PermissionType, PlatformPermissionNode, UserType } from '@/types'

interface TreeOption {
  title: string
  value: number
  children?: TreeOption[]
}

interface PermissionFormValues {
  parent_id?: number
  code: string
  name: string
  type: PermissionType
  scope: UserType
  path?: string
  icon?: string
  sort: number
}

/** 图标候选与 layouts/menu.tsx 的 iconMap 对齐；未命中时菜单渲染会兜底默认图标 */
const iconOptions = [
  { label: '工作台 dashboard', value: 'dashboard' },
  { label: '店铺 shop', value: 'shop' },
  { label: '团队 team', value: 'team' },
  { label: '安全 safety', value: 'safety' },
  { label: '应用 appstore', value: 'appstore' },
  { label: '设置 setting', value: 'setting' },
  { label: '用户 user', value: 'user' },
  { label: '日志 log', value: 'log' },
  { label: '配置 config', value: 'config' },
]

function toTreeOptions(nodes: PlatformPermissionNode[], excludeId?: number): TreeOption[] {
  return nodes
    .filter((n) => n.id !== excludeId)
    .map((n) => ({
      title: `${n.name}（${n.code}）`,
      value: n.id,
      children: n.children?.length ? toTreeOptions(n.children, excludeId) : undefined,
    }))
}

/** 平台端：权限点（菜单/按钮）可视化配置 */
export default function PermissionPage() {
  const actionRef = useRef<ActionType>(null)
  const [open, setOpen] = useState(false)
  const message = useMessage()
  const [editing, setEditing] = useState<PlatformPermissionNode | null>(null)
  const [tree, setTree] = useState<PlatformPermissionNode[]>([])

  const columns: ProColumns<PlatformPermissionNode>[] = [
    { title: '名称', dataIndex: 'name', width: 200 },
    { title: '编码', dataIndex: 'code', width: 220 },
    {
      title: '类型',
      dataIndex: 'type',
      width: 90,
      render: (_, r) => (r.type === 'menu' ? <Tag color="blue">菜单</Tag> : <Tag>按钮</Tag>),
    },
    {
      title: '归属',
      dataIndex: 'scope',
      width: 90,
      render: (_, r) => (r.scope === 'platform' ? <Tag color="gold">平台</Tag> : <Tag>租户</Tag>),
    },
    { title: '路由路径', dataIndex: 'path', ellipsis: true },
    { title: '图标', dataIndex: 'icon', width: 100 },
    { title: '排序', dataIndex: 'sort', width: 80 },
    {
      title: '操作',
      valueType: 'option',
      key: 'option',
      width: 140,
      render: (_, record) => [
        <PermButton
          key="edit"
          perm="platform.permission.update"
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
          title="确认删除该权限点？"
          description="删除后对应菜单或接口权限将失效，已授权角色下次保存时自动丢弃；此操作影响所有租户。"
          onConfirm={async () => {
            await permissionApi.remove(record.id)
            message.success('已删除')
            actionRef.current?.reload()
          }}
        >
          <PermButton perm="platform.permission.delete" type="link" size="small" danger>
            删除
          </PermButton>
        </Popconfirm>,
      ],
    },
  ]

  return (
    <PageContainer title="菜单配置">
      <DataTable<PlatformPermissionNode>
        headerTitle="权限点字典"
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        pagination={false}
        scroll={{ x: 1100 }}
        request={async () => {
          const data = await permissionApi.tree()
          setTree(data ?? [])
          return { data: data ?? [], success: true }
        }}
        toolBarRender={() => [
          <PermButton
            key="create"
            perm="platform.permission.create"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null)
              setOpen(true)
            }}
          >
            新建权限点
          </PermButton>,
        ]}
      />
      <DataDrawer<PermissionFormValues>
        key={editing?.id ?? 'create'}
        title={editing ? `编辑权限点 - ${editing.name}` : '新建权限点'}
        open={open}
        onOpenChange={setOpen}
        initialValues={
          editing ?? {
            type: 'menu',
            scope: 'platform',
            sort: 0,
            parent_id: undefined,
            icon: 'setting',
          }
        }
        onFinish={async (values) => {
          const payload = { ...values, parent_id: values.parent_id ?? 0 }
          if (editing) {
            await permissionApi.update(editing.id, payload)
            message.success('保存成功')
          } else {
            await permissionApi.create(payload)
            message.success('创建成功')
          }
          actionRef.current?.reload()
          return true
        }}
      >
        <ProFormTreeSelect
          name="parent_id"
          label="父节点"
          placeholder="不选则作为根节点"
          fieldProps={{
            treeData: toTreeOptions(tree, editing?.id),
            treeDefaultExpandAll: true,
            allowClear: true,
          }}
        />
        <ProFormText
          name="code"
          label="权限点编码"
          rules={[{ required: true, message: '请输入权限点编码' }]}
          placeholder="如 platform.example"
        />
        <ProFormText name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]} />
        <ProFormSelect
          name="type"
          label="类型"
          rules={[{ required: true }]}
          options={[
            { label: '菜单', value: 'menu' },
            { label: '按钮', value: 'button' },
          ]}
        />
        <ProFormSelect
          name="scope"
          label="归属"
          rules={[{ required: true }]}
          options={[
            { label: '平台', value: 'platform' },
            { label: '租户', value: 'tenant' },
          ]}
        />
        <ProFormText
          name="path"
          label="路由路径"
          placeholder="菜单类型填写，如 /platform/example"
        />
        <ProFormSelect name="icon" label="图标" options={iconOptions} allowClear />
        <ProFormDigit
          name="sort"
          label="排序"
          min={0}
          rules={[{ required: true, message: '请输入排序值' }]}
        />
      </DataDrawer>
    </PageContainer>
  )
}
