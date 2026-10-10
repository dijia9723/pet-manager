import { useRef, useState } from 'react'
import {
  Alert,
  Button,
  Descriptions,
  Drawer,
  Form,
  Input,
  Space,
  Tag,
  Tabs,
  Typography,
} from 'antd'
import { PageContainer } from '@/components/PageContainer'
import { DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { messageApi, type InboxItem } from '@/api/notification'
import { useAuthStore } from '@/store/auth'
import { useMessage } from '@/hooks/useMessage'
import { refreshUnreadCount } from '@/utils/notifications'
import { NotificationManager } from '@/components/biz/NotificationManager'
import { usePermission } from '@/hooks/usePermission'
import { MessageAttachments } from '@/components/biz/MessageAttachments'

export default function MessagesPage() {
  const userType = useAuthStore((s) => s.user?.user_type)
  const { has } = usePermission()
  return (
    <PageContainer>
      {userType === 'tenant' && has('tenant.notification.list') ? (
        <Tabs
          destroyOnHidden
          items={[
            { key: 'received', label: '收到的消息', children: <MessageInbox /> },
            {
              key: 'published',
              label: '发布消息',
              children: <NotificationManager scope="tenant" />,
            },
          ]}
        />
      ) : (
        <MessageInbox />
      )}
    </PageContainer>
  )
}

function MessageInbox() {
  const userType = useAuthStore((s) => s.user?.user_type ?? 'tenant')
  const api = messageApi(userType)
  const actionRef = useRef<ActionType>(null)
  const [detail, setDetail] = useState<InboxItem | null>(null)
  const [busy, setBusy] = useState(false)
  const [form] = Form.useForm<{ feedback: string }>()
  const message = useMessage()
  const reload = () => {
    actionRef.current?.reload()
    refreshUnreadCount()
  }
  const read = async (r: InboxItem) => {
    if (!r.read_at) {
      await api.read(r.id)
      reload()
    }
  }
  const showDetail = async (r: InboxItem) => {
    await api.read(r.id)
    reload()
    form.resetFields()
    setDetail({ ...r, read_at: r.read_at ?? new Date().toISOString() })
  }
  const columns: ProColumns<InboxItem>[] = [
    {
      title: '关键词',
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: { placeholder: '通知标题' },
    },
    {
      title: '标题',
      dataIndex: 'title',
      search: false,
      render: (_, r) => (
        <Button
          type="link"
          title={r.title}
          style={{
            padding: 0,
            fontWeight: r.read_at ? 400 : 600,
            maxWidth: 260,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
          onClick={() => {
            void showDetail(r).catch(() => {})
          }}
        >
          {r.title}
        </Button>
      ),
    },
    {
      title: '类型',
      dataIndex: 'category',
      search: false,
      valueEnum: { announcement: '平台公告', maintenance: '维护通知', business: '业务通知' },
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      search: false,
      valueEnum: { normal: '普通', urgent: { text: '紧急', status: 'Error' } },
    },
    {
      title: '消息状态',
      dataIndex: 'state',
      hideInTable: true,
      valueEnum: { unread: '未读', read: '已读', pending: '待处理', processed: '已处理' },
    },
    {
      title: '阅读 / 处理',
      key: 'stateLabel',
      search: false,
      render: (_, r) => (
        <Space>
          <Tag color={r.read_at ? 'default' : 'blue'}>{r.read_at ? '已读' : '未读'}</Tag>
          {r.require_process && (
            <Tag color={r.processed_at ? 'green' : 'orange'}>
              {r.processed_at ? '已处理' : '待处理'}
            </Tag>
          )}
        </Space>
      ),
    },
    { title: '发送人', dataIndex: 'sender_name', search: false },
    { title: '发布时间', dataIndex: 'published_at', valueType: 'dateTime', search: false },
    {
      title: '操作',
      valueType: 'option',
      render: (_, r) => [
        <Button
          key="view"
          type="link"
          size="small"
          onClick={() => {
            void showDetail(r).catch(() => {})
          }}
        >
          {r.require_process && !r.processed_at ? '查看并处理' : '查看'}
        </Button>,
        !r.read_at && (
          <Button
            key="read"
            type="link"
            size="small"
            onClick={() => {
              void read(r).catch(() => {})
            }}
          >
            标记已读
          </Button>
        ),
      ],
    },
  ]
  return (
    <>
      <DataTable<InboxItem>
        rowKey="id"
        actionRef={actionRef}
        headerTitle="消息中心"
        columns={columns}
        request={async (p) => {
          const data = await api.list({
            page: p.current,
            page_size: p.pageSize,
            keyword: p.keyword,
            state: p.state,
          })
          return { data: data.list, total: data.total, success: true }
        }}
      />
      <Drawer
        open={!!detail}
        onClose={() => setDetail(null)}
        title="消息详情"
        width={640}
        destroyOnHidden
      >
        {detail && (
          <>
            <Typography.Title level={4}>{detail.title}</Typography.Title>
            <Descriptions
              column={1}
              items={[
                { key: 'sender', label: '发送人', children: detail.sender_name },
                {
                  key: 'time',
                  label: '发布时间',
                  children: detail.published_at
                    ? new Date(detail.published_at).toLocaleString('zh-CN')
                    : '—',
                },
              ]}
            />
            <Typography.Paragraph
              style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', marginTop: 24 }}
            >
              {detail.content}
            </Typography.Paragraph>
            <MessageAttachments
              scope={userType}
              notificationId={detail.id}
              attachments={detail.attachments}
            />
            {detail.processed_at ? (
              <>
                <Alert type="success" showIcon message="已提交处理结果" />
                <Typography.Paragraph
                  style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', marginTop: 16 }}
                >
                  {detail.feedback}
                </Typography.Paragraph>
              </>
            ) : detail.require_process ? (
              <Form
                form={form}
                layout="vertical"
                onFinish={async ({ feedback }) => {
                  setBusy(true)
                  try {
                    await api.process(detail.id, feedback.trim())
                    message.success('处理结果已提交')
                    setDetail(null)
                    reload()
                  } finally {
                    setBusy(false)
                  }
                }}
              >
                <Alert
                  type="warning"
                  showIcon
                  message="该通知需要处理，请完成相关事项后提交处理结果。"
                  style={{ marginBottom: 16 }}
                />
                <Form.Item
                  name="feedback"
                  label="处理结果"
                  rules={[{ required: true, whitespace: true, message: '请填写处理结果' }]}
                >
                  <Input.TextArea rows={4} maxLength={2000} showCount />
                </Form.Item>
                <Button type="primary" htmlType="submit" loading={busy}>
                  提交处理结果
                </Button>
              </Form>
            ) : (
              <Alert type="info" showIcon message="该通知无需提交处理结果。" />
            )}
          </>
        )}
      </Drawer>
    </>
  )
}
