import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Form, List, Select, Space, Typography, Upload } from 'antd'
import { PaperClipOutlined, UploadOutlined } from '@ant-design/icons'
import { http } from '@/api/client'
import { formatBytes, openDownload, storageApi, type Attachment } from '@/api/storage'
import type { UserType } from '@/types'
import { useMessage } from '@/hooks/useMessage'

export function MessageAttachments({
  scope,
  notificationId,
  attachments,
  management = false,
}: {
  scope: UserType
  notificationId: number
  attachments?: Attachment[] | null
  management?: boolean
}) {
  const [busy, setBusy] = useState<number | null>(null)
  if (!attachments?.length) return null
  return (
    <List
      size="small"
      style={{ marginBottom: 24 }}
      header={
        <Space>
          <PaperClipOutlined />
          附件
        </Space>
      }
      dataSource={attachments}
      renderItem={(file) => (
        <List.Item
          actions={[
            <Button
              key="download"
              type="link"
              loading={busy === file.resource_id}
              onClick={async () => {
                setBusy(file.resource_id)
                try {
                  const result = await http.get<{ url: string }>(
                    `/${scope}/${management ? 'notifications' : 'messages'}/${notificationId}/attachments/${file.resource_id}/download`,
                  )
                  openDownload(result.url)
                } finally {
                  setBusy(null)
                }
              }}
            >
              下载
            </Button>,
          ]}
        >
          <Typography.Text style={{ overflowWrap: 'anywhere' }}>
            {file.name}（{formatBytes(file.size_bytes)}）
          </Typography.Text>
        </List.Item>
      )}
    />
  )
}

export function AttachmentPicker({
  scope,
  value,
  onChange,
  onBusyChange,
}: {
  scope: UserType
  value: Attachment[]
  onChange: (files: Attachment[]) => void
  onBusyChange: (busy: boolean) => void
}) {
  const api = storageApi(scope)
  const queryClient = useQueryClient()
  const message = useMessage()
  const [uploading, setUploading] = useState(false)
  const [keyword, setKeyword] = useState('')
  const { data: summary } = useQuery({ queryKey: ['storage-summary', scope], queryFn: api.summary })
  const { data, refetch } = useQuery({
    queryKey: ['attachment-resources', scope, keyword],
    queryFn: () => api.list({ own: true, status: 'ready', keyword, page_size: 50 }),
  })
  return (
    <Form.Item label="附件">
      <Space direction="vertical" style={{ width: '100%' }}>
        <Select
          style={{ width: '100%' }}
          placeholder="选择已有文件，可按文件名搜索"
          showSearch
          filterOption={false}
          onSearch={setKeyword}
          value={undefined}
          disabled={uploading || value.length >= 10}
          options={(data?.list ?? [])
            .filter((r) => !value.some((f) => f.resource_id === r.id))
            .map((r) => ({ value: r.id, label: `${r.name}（${formatBytes(r.size_bytes)}）` }))}
          onSelect={(id) => {
            const r = data?.list.find((r) => r.id === id)
            if (r)
              onChange([
                ...value,
                {
                  resource_id: r.id,
                  name: r.name,
                  size_bytes: r.size_bytes,
                  content_type: r.content_type,
                },
              ])
          }}
        />
        <Upload
          showUploadList={false}
          disabled={uploading || value.length >= 10 || !summary?.enabled}
          beforeUpload={async (file) => {
            if (!file.size || file.size > (summary?.max_upload_bytes ?? 0)) {
              message.error(
                `请选择非空文件，单文件不超过 ${formatBytes(summary?.max_upload_bytes ?? 0)}`,
              )
              return Upload.LIST_IGNORE
            }
            if (
              summary &&
              !summary.unlimited &&
              file.size > summary.quota_bytes - summary.used_bytes
            ) {
              message.error('存储空间不足，请联系平台升级套餐或扩容')
              return Upload.LIST_IGNORE
            }
            setUploading(true)
            onBusyChange(true)
            try {
              const r = await api.upload(file)
              onChange([
                ...value,
                {
                  resource_id: r.id,
                  name: r.name,
                  size_bytes: r.size_bytes,
                  content_type: r.content_type,
                },
              ])
              await refetch()
              await queryClient.invalidateQueries({ queryKey: ['storage-summary', scope] })
              message.success('附件已上传')
            } finally {
              setUploading(false)
              onBusyChange(false)
            }
            return false
          }}
        >
          <Button
            icon={<UploadOutlined />}
            loading={uploading}
            disabled={value.length >= 10 || !summary?.enabled}
          >
            上传新附件
          </Button>
        </Upload>
        <Typography.Text type="secondary">
          最多 10 个附件。文件统一存入平台
          TOS，按归属空间计量；移除附件只解除本消息引用，文件可在存储资源页删除。
        </Typography.Text>
        {!summary?.enabled && summary && (
          <Typography.Text type="warning">平台存储尚未启用</Typography.Text>
        )}
        {value.length > 0 && (
          <List
            size="small"
            dataSource={value}
            renderItem={(file) => (
              <List.Item
                actions={[
                  <Button
                    key="remove"
                    type="link"
                    disabled={uploading}
                    onClick={() =>
                      onChange(value.filter((f) => f.resource_id !== file.resource_id))
                    }
                  >
                    移除
                  </Button>,
                ]}
              >
                {file.name}（{formatBytes(file.size_bytes)}）
              </List.Item>
            )}
          />
        )}
      </Space>
    </Form.Item>
  )
}
