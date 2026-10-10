import { useRef, useState } from 'react'
import { PageContainer } from '@/components/PageContainer'
import { ProFormText } from '@ant-design/pro-components'
import { PlusOutlined } from '@ant-design/icons'
import { Alert, Popconfirm, Tag, Typography } from 'antd'
import dayjs from 'dayjs'
import { tenantApi } from '@/api/tenant'
import { PermButton } from '@/components/PermButton'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { useMessage } from '@/hooks/useMessage'
import type { Tenant, TenantCreatePayload, TenantStatus } from '@/types'

export default function TenantListPage() {
  const actionRef = useRef<ActionType>(null)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Tenant | null>(null)
  const [destroying, setDestroying] = useState<Tenant | null>(null)
  const message = useMessage()

  const columns: ProColumns<Tenant>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '租户名称 / 编码' },
    },
    { title: '租户名称', dataIndex: 'name', search: false },
    { title: '租户编码', dataIndex: 'code', search: false },
    { title: '联系人', dataIndex: 'contact_name', search: false },
    { title: '联系电话', dataIndex: 'contact_phone', search: false },
    { title: '过期时间', dataIndex: 'expire_at', valueType: 'dateTime', search: false },
    {
      title: '状态',
      dataIndex: 'status',
      valueEnum: {
        enabled: { text: '已启用', status: 'Success' },
        disabled: { text: '已停用', status: 'Default' },
        deleted: { text: '待销毁', status: 'Error' },
      },
      render: (_, record) => {
        if (record.status === 'deleted') {
          return (
            <div>
              <Tag color="red">待销毁</Tag>
              {record.purge_at && (
                <div
                  style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)', marginTop: 2 }}
                >
                  {dayjs(record.purge_at).format('YYYY-MM-DD HH:mm')} 自动销毁
                </div>
              )}
            </div>
          )
        }
        return (
          <Tag color={record.status === 'enabled' ? 'green' : 'default'}>
            {record.status === 'enabled' ? '已启用' : '已停用'}
          </Tag>
        )
      },
    },
    {
      title: '操作',
      valueType: 'option',
      key: 'option',
      width: 330,
      fixed: 'right',
      render: (_, record) => {
        const destroyButton = (
          <PermButton
            key="destroy"
            perm="platform.tenant.destroy"
            type="link"
            size="small"
            danger
            onClick={() => setDestroying(record)}
          >
            立即销毁
          </PermButton>
        )
        if (record.status === 'deleted') {
          return [
            <PermButton
              key="restore"
              perm="platform.tenant.delete"
              type="link"
              size="small"
              onClick={async () => {
                await tenantApi.restore(record.id)
                message.success('已撤销删除，租户恢复为停用状态，可手动启用')
                actionRef.current?.reload()
              }}
            >
              撤销删除
            </PermButton>,
            destroyButton,
          ]
        }
        return [
          <PermButton
            key="edit"
            perm="platform.tenant.update"
            type="link"
            size="small"
            onClick={() => {
              setEditing(record)
              setOpen(true)
            }}
          >
            编辑
          </PermButton>,
          record.status === 'enabled' ? (
            <PermButton
              key="disable"
              perm="platform.tenant.update"
              type="link"
              size="small"
              onClick={async () => {
                await tenantApi.disable(record.id)
                message.success('已停用')
                actionRef.current?.reload()
              }}
            >
              停用
            </PermButton>
          ) : (
            <PermButton
              key="enable"
              perm="platform.tenant.update"
              type="link"
              size="small"
              onClick={async () => {
                await tenantApi.enable(record.id)
                message.success('已启用')
                actionRef.current?.reload()
              }}
            >
              启用
            </PermButton>
          ),
          <Popconfirm
            key="delete"
            title="确定删除该租户？"
            description="删除后租户立即停用，数据保留 7 天，到期由系统自动物理删除，期间可撤销。"
            okText="确认删除"
            cancelText="再想想"
            okButtonProps={{ danger: true }}
            onConfirm={async () => {
              await tenantApi.remove(record.id)
              message.success('已删除，7 天后物理销毁数据，期间可撤销')
              actionRef.current?.reload()
            }}
          >
            <PermButton perm="platform.tenant.delete" type="link" size="small" danger>
              删除
            </PermButton>
          </Popconfirm>,
          destroyButton,
        ]
      },
    },
  ]

  return (
    <PageContainer title="租户管理">
      <DataTable<Tenant>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        scroll={{ x: 1450 }}
        request={async (params) => {
          const res = await tenantApi.list({
            page: params.current,
            page_size: params.pageSize,
            keyword: params.keyword,
            status: params.status as TenantStatus | undefined,
          })
          return { data: res?.list ?? [], total: res?.total ?? 0, success: true }
        }}
        toolBarRender={() => [
          <PermButton
            key="create"
            perm="platform.tenant.create"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null)
              setOpen(true)
            }}
          >
            新建租户
          </PermButton>,
        ]}
      />

      {destroying && (
        <DataDrawer<{ confirm_code: string }>
          key={`destroy-${destroying.id}`}
          title="立即销毁租户"
          open
          onOpenChange={(value) => {
            if (!value) setDestroying(null)
          }}
          submitter={{
            searchConfig: { submitText: '确认立即销毁' },
            submitButtonProps: { danger: true },
          }}
          onFinish={async (values) => {
            const result = await tenantApi.destroy(destroying.id, values.confirm_code)
            message.success(
              result.pending_resources
                ? `租户已立即销毁，${result.pending_resources} 个存储文件将由后台清理`
                : '租户已永久销毁',
            )
            setDestroying(null)
            actionRef.current?.reload()
            return true
          }}
        >
          <Alert
            type="error"
            showIcon
            message="此操作不可撤销"
            description="立即删除租户业务数据、账号、消息和租户记录，跳过删除宽限期。存储文件由后台自动清理，失败会重试。"
            style={{ marginBottom: 24 }}
          />
          <Typography.Paragraph>
            租户：<strong>{destroying.name}</strong>
          </Typography.Paragraph>
          <Typography.Paragraph>
            请输入租户编码 <Typography.Text code>{destroying.code}</Typography.Text> 确认销毁。
          </Typography.Paragraph>
          <ProFormText
            name="confirm_code"
            label="确认租户编码"
            placeholder={destroying.code}
            fieldProps={{ autoComplete: 'off' }}
            rules={[
              { required: true, message: '请输入租户编码' },
              {
                validator: (_, value) =>
                  value === destroying.code
                    ? Promise.resolve()
                    : Promise.reject(new Error('租户编码不匹配')),
              },
            ]}
          />
        </DataDrawer>
      )}

      <DataDrawer<TenantCreatePayload>
        key={editing?.id ?? 'create'}
        title={editing ? '编辑租户' : '新建租户'}
        open={open}
        onOpenChange={setOpen}
        initialValues={editing ?? { status: 'enabled' }}
        onFinish={async (values) => {
          if (editing) {
            await tenantApi.update(editing.id, {
              name: values.name,
              code: values.code,
              contact_name: values.contact_name,
              contact_phone: values.contact_phone,
            })
            message.success('保存成功')
          } else {
            await tenantApi.create(values)
            message.success('租户已创建，将自动开通 Schema 与主账号')
          }
          actionRef.current?.reload()
          return true
        }}
      >
        <ProFormText
          name="name"
          label="租户名称"
          placeholder="如：萌宠生活馆"
          rules={[{ required: true, message: '请输入租户名称' }]}
        />
        <ProFormText
          name="code"
          label="租户编码"
          placeholder="租户登录时使用，创建后不可修改，如：mengchong01"
          rules={[{ required: true, message: '请输入租户编码' }]}
          disabled={!!editing}
        />
        <ProFormText name="contact_name" label="联系人" placeholder="选填" />
        <ProFormText name="contact_phone" label="联系电话" placeholder="选填" />
        {!editing && (
          <>
            <ProFormText
              name={['owner', 'username']}
              label="主账号"
              placeholder="3-64 位，租户端登录用户名"
              rules={[{ required: true, message: '请输入主账号' }]}
            />
            <ProFormText.Password
              name={['owner', 'password']}
              label="主账号密码"
              placeholder="至少 6 位"
              rules={[{ required: true, message: '请输入主账号密码' }]}
            />
            <ProFormText name={['owner', 'nickname']} label="主账号昵称" placeholder="选填" />
          </>
        )}
      </DataDrawer>
    </PageContainer>
  )
}
