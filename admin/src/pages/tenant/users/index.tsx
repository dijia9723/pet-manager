import { useRef, useState } from 'react'
import { PageContainer } from '@/components/PageContainer'
import { ProFormSelect, ProFormText } from '@ant-design/pro-components'
import { PlusOutlined } from '@ant-design/icons'
import { Button, Popconfirm, Tag } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { rbacApi } from '@/api/rbac'
import { userApi } from '@/api/user'
import { PermButton } from '@/components/PermButton'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { useMessage } from '@/hooks/useMessage'
import { usePermission } from '@/hooks/usePermission'
import type { Role, TenantUser } from '@/types'

interface UserFormValues {
  username?: string
  password?: string
  nickname: string
  role_ids: number[]
  status?: string
}

export default function UserListPage() {
  const actionRef = useRef<ActionType>(null)
  const [open, setOpen] = useState(false)
  const message = useMessage()
  const [editing, setEditing] = useState<TenantUser | null>(null)
  const { has } = usePermission()

  const { data: roles = [] } = useQuery<Role[]>({
    queryKey: ['tenant-roles'],
    queryFn: () => rbacApi.listRoles(),
  })

  const roleOptions = roles.map((role) => ({ label: role.name, value: role.id }))

  const columns: ProColumns<TenantUser>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '账号 / 昵称' },
    },
    { title: '账号', dataIndex: 'username', search: false },
    { title: '昵称', dataIndex: 'nickname', search: false },
    {
      title: '角色',
      dataIndex: 'role_names',
      search: false,
      render: (_, record) =>
        (record.role_names ?? []).length > 0
          ? record.role_names.map((name) => <Tag key={name}>{name}</Tag>)
          : '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      search: false,
      render: (_, record) => (
        <Tag color={record.status === 'disabled' ? 'default' : 'green'}>
          {record.status === 'disabled' ? '已停用' : '正常'}
        </Tag>
      ),
    },
    { title: '创建时间', dataIndex: 'created_at', valueType: 'dateTime', search: false },
    {
      title: '操作',
      valueType: 'option',
      key: 'option',
      render: (_, record) => [
        <PermButton
          key="edit"
          perm="tenant.user.update"
          type="link"
          size="small"
          onClick={() => {
            setEditing(record)
            setOpen(true)
          }}
        >
          编辑
        </PermButton>,
        has('tenant.user.reset') ? (
          <DataDrawer<{ password: string }>
            key={`reset-${record.id}`}
            width={420}
            title={`重置密码 - ${record.username}`}
            trigger={
              <Button type="link" size="small">
                重置密码
              </Button>
            }
            onFinish={async (values) => {
              await userApi.resetPassword(record.id, values.password)
              message.success('密码已重置')
              return true
            }}
          >
            <ProFormText.Password
              name="password"
              label="新密码"
              rules={[
                { required: true, message: '请输入新密码' },
                { min: 6, message: '密码至少 6 位' },
              ]}
            />
          </DataDrawer>
        ) : null,
        <Popconfirm
          key="delete"
          title="确认删除该子账号？"
          onConfirm={async () => {
            await userApi.remove(record.id)
            message.success('已删除')
            actionRef.current?.reload()
          }}
        >
          <PermButton perm="tenant.user.delete" type="link" size="small" danger>
            删除
          </PermButton>
        </Popconfirm>,
      ],
    },
  ]

  return (
    <PageContainer title="子账号管理">
      <DataTable<TenantUser>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        request={async (params) => {
          const res = await userApi.list({
            page: params.current,
            page_size: params.pageSize,
            keyword: params.keyword,
          })
          return { data: res?.list ?? [], total: res?.total ?? 0, success: true }
        }}
        toolBarRender={() => [
          <PermButton
            key="create"
            perm="tenant.user.create"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null)
              setOpen(true)
            }}
          >
            新建子账号
          </PermButton>,
        ]}
      />

      <DataDrawer<UserFormValues>
        key={editing?.id ?? 'create'}
        title={editing ? '编辑子账号' : '新建子账号'}
        open={open}
        onOpenChange={setOpen}
        initialValues={editing ?? { role_ids: [], status: 'enabled' }}
        onFinish={async (values) => {
          if (editing) {
            await userApi.update(editing.id, {
              nickname: values.nickname,
              role_ids: values.role_ids,
              status: values.status ?? editing.status,
            })
            message.success('保存成功')
          } else {
            await userApi.create({
              username: values.username ?? '',
              password: values.password ?? '',
              nickname: values.nickname,
              role_ids: values.role_ids,
            })
            message.success('创建成功')
          }
          actionRef.current?.reload()
          return true
        }}
      >
        {!editing && (
          <>
            <ProFormText
              name="username"
              label="账号"
              rules={[{ required: true, message: '请输入账号' }]}
            />
            <ProFormText.Password
              name="password"
              label="密码"
              rules={[
                { required: true, message: '请输入密码' },
                { min: 6, message: '密码至少 6 位' },
              ]}
            />
          </>
        )}
        <ProFormText
          name="nickname"
          label="昵称"
          rules={[{ required: true, message: '请输入昵称' }]}
        />
        <ProFormSelect
          name="role_ids"
          label="角色"
          mode="multiple"
          options={roleOptions}
          rules={[{ required: true, message: '请选择角色' }]}
        />
        {editing && (
          <ProFormSelect
            name="status"
            label="状态"
            options={[
              { label: '正常', value: 'enabled' },
              { label: '停用', value: 'disabled' },
            ]}
          />
        )}
      </DataDrawer>
    </PageContainer>
  )
}
