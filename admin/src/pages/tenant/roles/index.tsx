import { useRef, useState } from 'react'
import { PageContainer } from '@/components/PageContainer'
import { ProFormItem, ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import { PlusOutlined } from '@ant-design/icons'
import { Popconfirm, Tag, Tree } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { rbacApi } from '@/api/rbac'
import { PermButton } from '@/components/PermButton'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { useMessage } from '@/hooks/useMessage'
import type { PermissionNode, Role, RolePayload } from '@/types'

interface PermissionTreeNode {
  key: number
  title: string
  children?: PermissionTreeNode[]
}

function toTreeData(nodes: PermissionNode[]): PermissionTreeNode[] {
  return nodes.map((node) => ({
    key: node.id,
    title: `${node.name}（${node.code}）`,
    children: node.children?.length ? toTreeData(node.children) : undefined,
  }))
}

export default function RoleListPage() {
  const actionRef = useRef<ActionType>(null)
  const [open, setOpen] = useState(false)
  const message = useMessage()
  const [editing, setEditing] = useState<Role | null>(null)

  const { data: permissions = [] } = useQuery<PermissionNode[]>({
    queryKey: ['tenant-permissions'],
    queryFn: () => rbacApi.permissions(),
  })
  const treeData = toTreeData(permissions)

  const columns: ProColumns<Role>[] = [
    { title: '角色名称', dataIndex: 'name' },
    { title: '角色编码', dataIndex: 'code' },
    {
      title: '类型',
      dataIndex: 'is_system',
      search: false,
      render: (_, record) =>
        record.is_system ? <Tag color="gold">系统内置</Tag> : <Tag>自定义</Tag>,
    },
    { title: '用户数', dataIndex: 'user_count', search: false },
    {
      title: '权限点数',
      dataIndex: 'permission_ids',
      search: false,
      render: (_, record) => record.permission_ids?.length ?? 0,
    },
    { title: '备注', dataIndex: 'remark', search: false, ellipsis: true },
    {
      title: '操作',
      valueType: 'option',
      key: 'option',
      render: (_, record) => [
        <PermButton
          key="edit"
          perm="tenant.role.update"
          type="link"
          size="small"
          onClick={() => {
            setEditing(record)
            setOpen(true)
          }}
        >
          编辑授权
        </PermButton>,
        !record.is_system ? (
          <Popconfirm
            key="delete"
            title="确认删除该角色？"
            onConfirm={async () => {
              await rbacApi.removeRole(record.id)
              message.success('已删除')
              actionRef.current?.reload()
            }}
          >
            <PermButton perm="tenant.role.delete" type="link" size="small" danger>
              删除
            </PermButton>
          </Popconfirm>
        ) : null,
      ],
    },
  ]

  return (
    <PageContainer title="角色授权">
      <DataTable<Role>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        pagination={false}
        request={async () => {
          const list = await rbacApi.listRoles()
          return { data: list ?? [], success: true }
        }}
        toolBarRender={() => [
          <PermButton
            key="create"
            perm="tenant.role.create"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null)
              setOpen(true)
            }}
          >
            新建角色
          </PermButton>,
        ]}
      />

      <DataDrawer<RolePayload>
        width={560}
        key={editing?.id ?? 'create'}
        title={editing ? `编辑角色 - ${editing.name}` : '新建角色'}
        open={open}
        onOpenChange={setOpen}
        initialValues={editing ?? { permission_ids: [] }}
        onFinish={async (values) => {
          if (editing) {
            await rbacApi.updateRole(editing.id, values)
            message.success('保存成功')
          } else {
            await rbacApi.createRole(values)
            message.success('创建成功')
          }
          actionRef.current?.reload()
          return true
        }}
      >
        <ProFormText
          name="name"
          label="角色名称"
          rules={[{ required: true, message: '请输入角色名称' }]}
        />
        <ProFormText
          name="code"
          label="角色编码"
          rules={[{ required: true, message: '请输入角色编码' }]}
        />
        <ProFormTextArea name="remark" label="备注" />
        <ProFormItem
          name="permission_ids"
          label="权限点"
          valuePropName="checkedKeys"
          trigger="onCheck"
        >
          <Tree checkable selectable={false} defaultExpandAll treeData={treeData} />
        </ProFormItem>
      </DataDrawer>
    </PageContainer>
  )
}
