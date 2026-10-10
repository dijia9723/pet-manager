import { useRef, useState, type CSSProperties } from 'react'
import { Alert, App, Button, Drawer, Popconfirm, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import {
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components'
import { PageContainer } from '@/components/PageContainer'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { usePermission } from '@/hooks/usePermission'
import { serverApi, type ManagedServer, type ServerPayload } from '@/api/hosting'
import SSHTerminal from '@/components/biz/SSHTerminal'

export default function ServersPage() {
  const actionRef = useRef<ActionType>(null)
  const { message } = App.useApp()
  const { has } = usePermission()
  const [editing, setEditing] = useState<ManagedServer | null>(null)
  const [open, setOpen] = useState(false)
  const [terminal, setTerminal] = useState<ManagedServer | null>(null)
  const [tested, setTested] = useState<ManagedServer | null>(null)
  const [output, setOutput] = useState('')
  const [busy, setBusy] = useState(false)
  const columns: ProColumns<ManagedServer>[] = [
    { title: '关键字', dataIndex: 'keyword', hideInTable: true },
    { title: '服务器', dataIndex: 'name', search: false },
    { title: 'SSH 地址', search: false, render: (_, row) => `${row.host}:${row.port}` },
    { title: '账号', dataIndex: 'username', search: false },
    {
      title: '认证',
      dataIndex: 'auth_type',
      search: false,
      render: (_, row) => (row.auth_type === 'key' ? 'SSH 私钥' : '密码'),
    },
    {
      title: '状态',
      search: false,
      render: (_, row) => (
        <Tag color={row.status === 'enabled' ? 'green' : 'default'}>
          {row.status === 'enabled' ? '启用' : '停用'}
        </Tag>
      ),
    },
    { title: '备注', dataIndex: 'remark', search: false, ellipsis: true },
    {
      title: '操作',
      valueType: 'option',
      width: 240,
      render: (_, row) => [
        has('platform.server.update') && (
          <a
            key="edit"
            onClick={() => {
              setEditing(row)
              setOpen(true)
            }}
          >
            编辑
          </a>
        ),
        <a
          key="test"
          onClick={async () => {
            setTested(row)
            setOutput('正在测试连接…')
            setBusy(true)
            try {
              const result = await serverApi.test(row.id)
              setOutput(`${result.output}\n退出码：${result.exit_code}`)
            } catch (error) {
              setOutput(
                error instanceof Error
                  ? error.message
                  : '连接测试失败，请检查地址、凭证及主机指纹。',
              )
            } finally {
              setBusy(false)
            }
          }}
        >
          测试连接
        </a>,
        has('platform.server.exec') && (
          <a
            key="terminal"
            onClick={() => {
              setTerminal(row)
            }}
          >
            进入终端
          </a>
        ),
        has('platform.server.delete') && (
          <Popconfirm
            key="remove"
            title="删除服务器记录？"
            description="有关联网站时不能删除。"
            onConfirm={async () => {
              await serverApi.remove(row.id)
              message.success('已删除')
              actionRef.current?.reload()
            }}
          >
            <a style={{ color: 'var(--ant-color-error)' }}>删除</a>
          </Popconfirm>
        ),
      ],
    },
  ]
  return (
    <PageContainer title="服务器管理">
      <Alert
        type="info"
        showIcon
        message="平台通过 SSH 管理服务器。网站目录初始化与 CMS 部署支持 root 账号或具有免密 sudo 权限的账号。"
        style={{ marginBottom: 16 }}
      />
      <DataTable<ManagedServer>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        request={async (params) => {
          const result = await serverApi.list({
            page: params.current,
            page_size: params.pageSize,
            keyword: params.keyword,
          })
          return { data: result.list, total: result.total, success: true }
        }}
        toolBarRender={() =>
          has('platform.server.create')
            ? [
                <Button
                  key="add"
                  type="primary"
                  icon={<PlusOutlined />}
                  onClick={() => {
                    setEditing(null)
                    setOpen(true)
                  }}
                >
                  新增服务器
                </Button>,
              ]
            : []
        }
      />
      <DataDrawer<ServerPayload>
        key={editing?.id ?? 'new'}
        title={editing ? '编辑服务器' : '新增服务器'}
        open={open}
        onOpenChange={setOpen}
        initialValues={
          editing
            ? { ...editing, credential: '' }
            : { port: 22, username: 'root', auth_type: 'key', status: 'enabled' }
        }
        onFinish={async (values) => {
          if (editing) await serverApi.update(editing.id, values)
          else await serverApi.create(values)
          message.success('已保存')
          setOpen(false)
          actionRef.current?.reload()
          return true
        }}
      >
        <ProFormText name="name" label="服务器名称" rules={[{ required: true }, { max: 128 }]} />
        <ProFormText
          name="host"
          label="主机 / IP"
          rules={[{ required: true }]}
          placeholder="192.0.2.10 或 server.example.com"
        />
        <ProFormDigit
          name="port"
          label="SSH 端口"
          min={1}
          max={65535}
          rules={[{ required: true }]}
        />
        <ProFormText name="username" label="SSH 账号" rules={[{ required: true }]} />
        <ProFormSelect
          name="auth_type"
          label="认证方式"
          options={[
            { label: 'SSH 私钥', value: 'key' },
            { label: '密码', value: 'password' },
          ]}
          rules={[{ required: true }]}
        />
        <ProFormTextArea
          name="credential"
          label={editing ? '替换凭证（留空保留）' : '私钥 / 密码'}
          fieldProps={{
            rows: 4,
            autoComplete: 'off',
            style: { WebkitTextSecurity: 'disc' } as CSSProperties,
          }}
          rules={editing ? [] : [{ required: true }]}
        />
        <ProFormText
          name="host_fingerprint"
          label="SSH 主机指纹"
          placeholder="SHA256:… 或粘贴 ssh-keygen 输出"
          rules={[{ required: true }]}
          extra="通过可信渠道取得，例如服务器上的 ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub。支持纯指纹值、SHA256: 前缀或完整单行输出。已有网站时不可更换主机。"
        />
        <ProFormSelect
          name="status"
          label="状态"
          options={[
            { label: '启用', value: 'enabled' },
            { label: '停用', value: 'disabled' },
          ]}
        />
        <ProFormTextArea name="remark" label="备注" rules={[{ max: 255 }]} />
      </DataDrawer>
      <Drawer
        title={tested ? `连接测试 · ${tested.name}` : '连接测试'}
        width={760}
        open={!!tested}
        onClose={() => {
          if (!busy) setTested(null)
        }}
      >
        <Typography.Paragraph>
          <pre style={{ whiteSpace: 'pre-wrap', maxHeight: '60vh', overflow: 'auto' }}>
            {output}
          </pre>
        </Typography.Paragraph>
      </Drawer>
      <SSHTerminal server={terminal} onClose={() => setTerminal(null)} />
    </PageContainer>
  )
}
