import { useRef, useState } from 'react'
import { Alert, App, Button, Card, Popconfirm, Space, Tag, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import {
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components'
import { PageContainer } from '@/components/PageContainer'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { WebsiteFiles } from './WebsiteFiles'
import { usePermission } from '@/hooks/usePermission'
import { tenantApi } from '@/api/tenant'
import {
  serverApi,
  websiteApi,
  type Website,
  type WebsitePayload,
  type CMSDeployPayload,
} from '@/api/hosting'

const phaseLabels: Record<string, string> = {
  pending: '待初始化',
  provisioning: '初始化中',
  deploying: '部署中',
  ready: '就绪',
  failed: '失败 · 可重试',
}
const phaseTag = (value: string) => (
  <Tag
    color={
      value === 'ready'
        ? 'green'
        : value === 'failed'
          ? 'red'
          : value === 'deploying' || value === 'provisioning'
            ? 'processing'
            : 'default'
    }
  >
    {phaseLabels[value] || value}
  </Tag>
)
export function WebsiteManager({ scope }: { scope: 'platform' | 'tenant' }) {
  const platform = scope === 'platform'
  const api = websiteApi(scope)
  const { message } = App.useApp()
  const { has } = usePermission()
  const actionRef = useRef<ActionType>(null)
  const [editing, setEditing] = useState<Website | null>(null)
  const [open, setOpen] = useState(false)
  const [files, setFiles] = useState<Website | null>(null)
  const [deploying, setDeploying] = useState<Website | null>(null)
  const [busySite, setBusySite] = useState<number | null>(null)
  const columns: ProColumns<Website>[] = [
    { title: '关键字', dataIndex: 'keyword', hideInTable: true },
    { title: '网站', dataIndex: 'name', search: false },
    ...(platform
      ? [{ title: '租户 ID', dataIndex: 'tenant_id', valueType: 'digit' as const, width: 100 }]
      : []),
    {
      title: '网站地址',
      dataIndex: 'url',
      search: false,
      ellipsis: true,
      render: (_, row) =>
        row.url ? (
          <a href={row.url} target="_blank" rel="noopener noreferrer">
            {row.url}
          </a>
        ) : (
          '未配置'
        ),
    },
    { title: '目录状态', search: false, render: (_, row) => phaseTag(row.provision_status) },
    {
      title: 'CMS',
      search: false,
      render: (_, row) =>
        row.cms === 'directus' ? (
          <Space direction="vertical" size={0}>
            <Tag color="blue">Directus {row.cms_version}</Tag>
            {phaseTag(row.cms_status)}
          </Space>
        ) : (
          '无'
        ),
    },
    {
      title: '文件维护',
      search: false,
      render: (_, row) => (
        <Tag color={row.status === 'enabled' ? 'green' : 'default'}>
          {row.status === 'enabled' ? '启用' : '停用'}
        </Tag>
      ),
    },
    {
      title: '操作',
      valueType: 'option',
      width: platform ? 330 : 180,
      render: (_, row) => [
        platform && has('platform.website.update') && (
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
        has(`${scope}.website.files`) &&
          row.provision_status === 'ready' &&
          row.status === 'enabled' && (
            <a key="files" onClick={() => setFiles(row)}>
              文件管理
            </a>
          ),
        row.cms === 'directus' && row.cms_url && (
          <a key="cms" href={`${row.cms_url}/admin`} target="_blank" rel="noopener noreferrer">
            CMS 后台
          </a>
        ),
        platform &&
          has('platform.website.provision') &&
          ['pending', 'failed'].includes(row.provision_status) && (
            <Popconfirm
              key="provision"
              title="初始化隔离目录？"
              description="创建独立 SFTP 账号和目录，验证配置后重载 SSH 服务。"
              onConfirm={async () => {
                setBusySite(row.id)
                try {
                  await api.provision(row.id)
                  message.success('隔离目录已就绪')
                } finally {
                  setBusySite(null)
                  actionRef.current?.reload()
                }
              }}
              disabled={busySite !== null}
            >
              <a>{busySite === row.id ? '初始化中…' : '初始化目录'}</a>
            </Popconfirm>
          ),
        platform &&
          has('platform.website.deploy') &&
          row.cms === 'directus' &&
          row.provision_status === 'ready' &&
          ['pending', 'failed'].includes(row.cms_status) && (
            <a key="deploy" onClick={() => setDeploying(row)}>
              部署 CMS
            </a>
          ),
      ],
    },
  ]
  return (
    <PageContainer title="网站管理">
      <Alert
        showIcon
        type="info"
        style={{ marginBottom: 16 }}
        message={
          platform
            ? '分配网站后初始化隔离目录，再部署 Directus。网站地址和 CMS 域名需配置 DNS、反向代理与 HTTPS。'
            : '维护属于自己的站点文件；文章、页面、媒体和内容模型通过独立 Directus 后台管理。'
        }
      />
      <DataTable<Website>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        request={async (params) => {
          const result = await api.list({
            page: params.current,
            page_size: params.pageSize,
            keyword: params.keyword,
            tenant_id: params.tenant_id,
          })
          return { data: result.list, total: result.total, success: true }
        }}
        toolBarRender={() =>
          platform && has('platform.website.create')
            ? [
                <Button
                  key="add"
                  icon={<PlusOutlined />}
                  type="primary"
                  onClick={() => {
                    setEditing(null)
                    setOpen(true)
                  }}
                >
                  分配网站
                </Button>,
              ]
            : []
        }
      />
      <Card title="Headless CMS · Directus" style={{ marginTop: 16 }}>
        <Space direction="vertical">
          <Typography.Text>
            复用 Directus 的内容编辑器、媒体库、内容模型、角色权限与 REST / GraphQL API。
          </Typography.Text>
          <Typography.Text type="secondary">
            每个站点独立部署。网站前端通过 CMS API 获取内容，CMS 账号在对应 Directus 后台中管理。
          </Typography.Text>
          <Space wrap>
            <Typography.Link
              href="https://directus.com/docs/guides/content"
              target="_blank"
              rel="noopener noreferrer"
            >
              内容管理文档
            </Typography.Link>
            <Typography.Link
              href="https://directus.com/docs/api"
              target="_blank"
              rel="noopener noreferrer"
            >
              API 文档
            </Typography.Link>
          </Space>
        </Space>
      </Card>
      {platform && (
        <DataDrawer<WebsitePayload>
          key={editing?.id ?? 'new'}
          title={editing ? '编辑网站' : '分配网站'}
          open={open}
          onOpenChange={setOpen}
          initialValues={editing ?? { cms: 'directus', status: 'enabled' }}
          onFinish={async (values) => {
            if (editing) await api.update(editing.id, values)
            else await api.create(values)
            message.success('已保存')
            setOpen(false)
            actionRef.current?.reload()
            return true
          }}
        >
          <ProFormText name="name" label="网站名称" rules={[{ required: true }, { max: 128 }]} />
          <ProFormSelect
            name="tenant_id"
            label="所属租户"
            disabled={!!editing}
            rules={[{ required: true }]}
            showSearch
            request={async ({ keyWords }) => {
              const result = await tenantApi.list({ page_size: 50, keyword: keyWords })
              return result.list
                .filter((tenant) => tenant.status !== 'deleted')
                .map((tenant) => ({
                  label: `${tenant.name}（${tenant.code} / ID ${tenant.id}）`,
                  value: tenant.id,
                }))
            }}
          />
          <ProFormSelect
            name="server_id"
            label="服务器"
            disabled={!!editing}
            rules={[{ required: true }]}
            showSearch
            request={async ({ keyWords }) => {
              const result = await serverApi.list({ page_size: 50, keyword: keyWords })
              return result.list.map((server) => ({
                label: `${server.name}（${server.host}）`,
                value: server.id,
              }))
            }}
          />
          <ProFormText
            name="url"
            label="网站地址"
            placeholder="https://www.example.com"
            rules={[{ type: 'url' }]}
          />
          <ProFormSelect
            name="cms"
            label="CMS"
            options={[
              { label: 'Directus · Headless CMS', value: 'directus' },
              { label: '无 CMS', value: 'none' },
            ]}
            rules={[{ required: true }]}
          />
          <ProFormText
            name="cms_url"
            label="CMS 地址"
            placeholder="https://cms.example.com"
            rules={[{ type: 'url' }]}
            extra="填写 CMS 根地址，无需附加 /admin；独立域名便于接入网站前端。"
          />
          <ProFormSelect
            name="status"
            label="文件维护状态"
            options={[
              { label: '启用', value: 'enabled' },
              { label: '停用', value: 'disabled' },
            ]}
            rules={[{ required: true }]}
            extra="控制本平台文件操作入口；站点与 CMS 服务的启停由服务器管理控制。"
          />
          <ProFormTextArea name="remark" label="备注" rules={[{ max: 255 }]} />
        </DataDrawer>
      )}
      {files && (
        <WebsiteFiles key={files.id} site={files} scope={scope} onClose={() => setFiles(null)} />
      )}
      {deploying && (
        <DataDrawer<CMSDeployPayload>
          key={deploying.id}
          title={`部署 Directus · ${deploying.name}`}
          width={600}
          open
          initialValues={{ port: deploying.cms_port || 8055 }}
          onOpenChange={(open) => {
            if (!open) setDeploying(null)
          }}
          onFinish={async (values) => {
            try {
              await api.deploy(deploying.id, values)
              message.success('Directus 已部署，请完成 CMS 域名反向代理配置')
              setDeploying(null)
              return true
            } finally {
              actionRef.current?.reload()
            }
          }}
        >
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="服务器需安装 Docker Compose。CMS 仅监听本机端口，每站点使用独立数据库与数据卷。"
          />
          <ProFormDigit
            name="port"
            label="本机 CMS 端口"
            min={1024}
            max={65535}
            rules={[{ required: true }]}
            extra="不同站点使用不同端口，反向代理到 127.0.0.1:该端口。"
          />
          <ProFormText
            name="admin_email"
            label="CMS 管理员邮箱"
            rules={[{ required: true }, { type: 'email' }]}
          />
          <ProFormText.Password
            name="admin_password"
            label="初始管理员密码"
            rules={[{ required: true }, { min: 12 }, { max: 128 }]}
            extra="交付租户后在 CMS 中修改。重试部署保留数据，不会重置已有管理员密码。"
          />
          <ProFormText.Password name="license_key" label="Directus 许可证密钥（可选）" />
          <Typography.Paragraph type="secondary">
            本部署固定使用 Directus 12.5.0。商业托管须按{' '}
            <Typography.Link
              href="https://directus.com/license"
              target="_blank"
              rel="noopener noreferrer"
            >
              Directus 许可证
            </Typography.Link>{' '}
            核对授权范围。
          </Typography.Paragraph>
        </DataDrawer>
      )}
    </PageContainer>
  )
}
