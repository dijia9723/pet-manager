import { useRef, useState } from 'react'
import { Alert, Descriptions, Drawer, Popconfirm, Typography } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import {
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  type ProFormInstance,
} from '@ant-design/pro-components'
import { PermButton } from '@/components/PermButton'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import {
  createNotificationApi,
  type NotificationInput,
  type NotificationItem,
  type Receipt,
  type TargetType,
} from '@/api/notification'
import { useMessage } from '@/hooks/useMessage'
import { refreshUnreadCount } from '@/utils/notifications'
import type { UserType } from '@/types'
import { AttachmentPicker, MessageAttachments } from './MessageAttachments'
import type { Attachment } from '@/api/storage'

const platformTargets = {
  all_tenants: '全部租户账号',
  tenants: '指定租户的全部账号',
  tenant_users: '指定租户账号',
  platform_users: '指定平台账号',
}
const tenantTargets = { own_tenant: '本租户全部账号', tenant_users: '本租户指定账号' }
const targetsFor = (scope: UserType): Partial<Record<TargetType, string>> =>
  scope === 'tenant' ? tenantTargets : platformTargets
const states = {
  draft: { text: '草稿', status: 'Default' },
  published: { text: '已发布', status: 'Success' },
  revoked: { text: '已撤回', status: 'Warning' },
}

export function NotificationManager({ scope }: { scope: UserType }) {
  const notificationApi = createNotificationApi(scope)
  const permPrefix = `${scope}.notification`
  const targets = targetsFor(scope)
  const actionRef = useRef<ActionType>(null)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<NotificationItem | null>(null)
  const [detail, setDetail] = useState<NotificationItem | null>(null)
  const message = useMessage()
  const columns: ProColumns<NotificationItem>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '通知标题' },
    },
    { title: '通知标题', dataIndex: 'title', search: false, ellipsis: true },
    {
      title: '优先级',
      dataIndex: 'priority',
      search: false,
      valueEnum: { normal: '普通', urgent: { text: '紧急', status: 'Error' } },
    },
    { title: '接收范围', dataIndex: 'target_type', search: false, valueEnum: targets },
    { title: '状态', dataIndex: 'status', valueEnum: states },
    {
      title: '已读 / 接收',
      key: 'read',
      search: false,
      render: (_, r) => `${r.read_count} / ${r.recipient_count}`,
    },
    {
      title: '已处理 / 接收',
      key: 'processed',
      search: false,
      render: (_, r) =>
        r.require_process ? `${r.processed_count} / ${r.recipient_count}` : '无需处理',
    },
    { title: '发布时间', dataIndex: 'published_at', valueType: 'dateTime', search: false },
    {
      title: '操作',
      valueType: 'option',
      width: 250,
      fixed: 'right',
      render: (_, r) => [
        <PermButton
          key="detail"
          perm={`${permPrefix}.list`}
          type="link"
          size="small"
          onClick={() => setDetail(r)}
        >
          详情 / 回执
        </PermButton>,
        r.status === 'draft' && (
          <PermButton
            key="edit"
            perm={`${permPrefix}.create`}
            type="link"
            size="small"
            onClick={() => {
              setEditing(r)
              setOpen(true)
            }}
          >
            编辑
          </PermButton>
        ),
        r.status === 'draft' && (
          <Popconfirm
            key="publish"
            title="发布后内容和接收范围将冻结，确认下发？"
            onConfirm={async () => {
              await notificationApi.publish(r.id)
              message.success('通知已下发')
              refreshUnreadCount()
              actionRef.current?.reload()
            }}
          >
            <PermButton perm={`${permPrefix}.publish`} type="link" size="small">
              发布
            </PermButton>
          </Popconfirm>
        ),
        r.status === 'published' && (
          <Popconfirm
            key="revoke"
            title="撤回后接收人将无法查看和处理，历史回执保留。"
            onConfirm={async () => {
              await notificationApi.revoke(r.id)
              message.success('通知已撤回')
              refreshUnreadCount()
              actionRef.current?.reload()
            }}
          >
            <PermButton perm={`${permPrefix}.revoke`} danger type="link" size="small">
              撤回
            </PermButton>
          </Popconfirm>
        ),
      ],
    },
  ]
  return (
    <>
      <DataTable<NotificationItem>
        rowKey="id"
        actionRef={actionRef}
        headerTitle={scope === 'tenant' ? '消息发布' : '信息通知'}
        columns={columns}
        scroll={{ x: 1200 }}
        request={async (p) => {
          const data = await notificationApi.list({
            page: p.current,
            page_size: p.pageSize,
            keyword: p.keyword,
            status: p.status,
          })
          return { data: data.list, total: data.total, success: true }
        }}
        toolBarRender={() => [
          <PermButton
            key="new"
            perm={`${permPrefix}.create`}
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null)
              setOpen(true)
            }}
          >
            新建通知
          </PermButton>,
        ]}
      />
      {open && (
        <NotificationForm
          scope={scope}
          editing={editing}
          onClose={() => setOpen(false)}
          onSaved={() => {
            setOpen(false)
            actionRef.current?.reload()
          }}
        />
      )}
      <Drawer
        title="通知详情与接收回执"
        width={960}
        open={!!detail}
        onClose={() => setDetail(null)}
        destroyOnHidden
      >
        {detail && (
          <>
            <Descriptions
              column={2}
              items={[
                { key: 'title', label: '标题', children: detail.title },
                { key: 'status', label: '状态', children: states[detail.status].text },
                { key: 'sender', label: '发送人', children: detail.sender_name },
                { key: 'target', label: '接收范围', children: targets[detail.target_type] },
              ]}
            />
            <Typography.Paragraph
              style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', marginTop: 24 }}
            >
              {detail.content}
            </Typography.Paragraph>
            <MessageAttachments
              scope={scope}
              notificationId={detail.id}
              attachments={detail.attachments}
              management
            />
            <DataTable<Receipt>
              key={detail.id}
              rowKey="id"
              headerTitle="接收回执"
              columns={[
                { title: '关键词', dataIndex: 'keyword', hideInTable: true },
                {
                  title: '租户',
                  dataIndex: 'tenant_name',
                  search: false,
                  renderText: (v, r) => (r.user_type === 'platform' ? '平台账号' : v),
                },
                { title: '接收账号', dataIndex: 'username', search: false },
                {
                  title: '状态',
                  dataIndex: 'state',
                  hideInTable: true,
                  valueEnum: {
                    unread: '未读',
                    read: '已读',
                    pending: '待处理',
                    processed: '已处理',
                  },
                },
                { title: '已读时间', dataIndex: 'read_at', valueType: 'dateTime', search: false },
                {
                  title: '处理时间',
                  dataIndex: 'processed_at',
                  valueType: 'dateTime',
                  search: false,
                },
                {
                  title: '处理反馈',
                  dataIndex: 'feedback',
                  search: false,
                  render: (_, r) => (
                    <span style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
                      {r.feedback || '—'}
                    </span>
                  ),
                },
              ]}
              request={async (p) => {
                const data = await notificationApi.receipts(detail.id, {
                  page: p.current,
                  page_size: p.pageSize,
                  keyword: p.keyword,
                  state: p.state,
                })
                return { data: data.list, total: data.total, success: true }
              }}
            />
          </>
        )}
      </Drawer>
    </>
  )
}

function NotificationForm({
  scope,
  editing,
  onClose,
  onSaved,
}: {
  scope: UserType
  editing: NotificationItem | null
  onClose: () => void
  onSaved: () => void
}) {
  const notificationApi = createNotificationApi(scope)
  const targets = targetsFor(scope)
  const formRef = useRef<ProFormInstance<NotificationInput>>(null)
  const [attachments, setAttachments] = useState<Attachment[]>(editing?.attachments ?? [])
  const [attachmentBusy, setAttachmentBusy] = useState(false)
  const [targetType, setTargetType] = useState<TargetType>(
    editing?.target_type ?? (scope === 'tenant' ? 'own_tenant' : 'all_tenants'),
  )
  const message = useMessage()
  return (
    <DataDrawer<NotificationInput>
      formRef={formRef}
      title={editing ? '编辑通知草稿' : '新建通知'}
      open
      onOpenChange={(v) => {
        if (!v) onClose()
      }}
      width={640}
      initialValues={
        editing ?? {
          category: 'announcement',
          priority: 'normal',
          target_type: scope === 'tenant' ? 'own_tenant' : 'all_tenants',
          require_process: false,
          target_ids: [],
        }
      }
      submitter={{
        searchConfig: { submitText: '保存草稿' },
        submitButtonProps: { disabled: attachmentBusy },
      }}
      onFinish={async (values) => {
        await notificationApi.save(
          {
            ...values,
            title: values.title.trim(),
            content: values.content.trim(),
            attachment_ids: attachments.map((f) => f.resource_id),
            target_ids:
              targetType === 'all_tenants' || targetType === 'own_tenant' ? [] : values.target_ids,
          },
          editing?.id,
        )
        message.success('草稿已保存，可在列表中发布')
        onSaved()
        return true
      }}
    >
      <ProFormText
        name="title"
        label="通知标题"
        rules={[{ required: true, whitespace: true }]}
        fieldProps={{ maxLength: 128, showCount: true }}
      />
      <ProFormSelect
        name="category"
        label="通知类型"
        valueEnum={{ announcement: '公告', maintenance: '维护通知', business: '业务通知' }}
        rules={[{ required: true }]}
      />
      <ProFormSelect
        name="priority"
        label="优先级"
        valueEnum={{ normal: '普通', urgent: '紧急' }}
        rules={[{ required: true }]}
      />
      <ProFormTextArea
        name="content"
        label="通知内容"
        rules={[{ required: true, whitespace: true }]}
        fieldProps={{ rows: 7, maxLength: 20000, showCount: true }}
      />
      <ProFormSelect
        name="target_type"
        label="接收范围"
        valueEnum={targets}
        rules={[{ required: true }]}
        fieldProps={{
          allowClear: false,
          onChange: (v) => {
            formRef.current?.setFieldValue('target_ids', [])
            setTargetType(v as TargetType)
          },
        }}
      />
      {targetType !== 'all_tenants' && targetType !== 'own_tenant' && (
        <ProFormSelect
          key={targetType}
          name="target_ids"
          label="接收对象"
          mode="multiple"
          rules={[{ required: true, message: '请选择接收对象' }]}
          params={{ targetType }}
          request={async ({ keyWords }) => {
            const data = await notificationApi.targets(targetType, keyWords)
            return data.list.map((r) => ({ value: r.id, label: r.label }))
          }}
          fieldProps={{
            showSearch: true,
            filterOption: false,
            maxCount: 500,
            placeholder:
              scope === 'tenant'
                ? '搜索本租户账号（最多选择 500 个）'
                : '输入租户名称或账号搜索（最多选择 500 个）',
          }}
        />
      )}
      <ProFormSwitch name="require_process" label="需要接收人处理并反馈" />
      <AttachmentPicker
        scope={scope}
        value={attachments}
        onChange={setAttachments}
        onBusyChange={setAttachmentBusy}
      />
      <Alert
        type="info"
        showIcon
        message={
          scope === 'tenant'
            ? '仅可向本租户的启用账号发布消息。保存草稿后在列表中发布，可查看回执或撤回。'
            : '保存后在列表中发布。仅向有效租户的启用账号下发，已发布通知可查看回执或撤回。'
        }
      />
    </DataDrawer>
  )
}
