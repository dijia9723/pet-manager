import { useRef, useState } from 'react'
import { App, Button, Drawer, Popconfirm, Space, Tag, Upload } from 'antd'
import { FolderOutlined, FileOutlined, UploadOutlined } from '@ant-design/icons'
import { ProFormText, ProFormTextArea } from '@ant-design/pro-components'
import { DataDrawer, DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { websiteApi, type FileItem, type Website } from '@/api/hosting'
import { usePermission } from '@/hooks/usePermission'

interface Props {
  site: Website
  scope: 'platform' | 'tenant'
  onClose: () => void
}
const join = (directory: string, name: string) => `${directory === '/' ? '' : directory}/${name}`
export function WebsiteFiles({ site, scope, onClose }: Props) {
  const api = websiteApi(scope)
  const { message, modal } = App.useApp()
  const { has } = usePermission()
  const writable = has(`${scope}.website.write`)
  const actionRef = useRef<ActionType>(null)
  const [directory, setDirectory] = useState('/')
  const [editor, setEditor] = useState<{ path: string; content: string } | null>(null)
  const [newFile, setNewFile] = useState(false)
  const [folderOpen, setFolderOpen] = useState(false)
  const [renaming, setRenaming] = useState<FileItem | null>(null)
  const [uploading, setUploading] = useState(false)
  const validName = [
    { required: true },
    { pattern: /^[^/\\]+$/, message: '请输入单个文件名' },
    {
      validator: (_: unknown, name: string) =>
        name === '.' ||
        name === '..' ||
        (name && ['\0', '\r', '\n'].some((char) => name.includes(char)))
          ? Promise.reject(new Error('名称不合法'))
          : Promise.resolve(),
    },
  ]
  const reload = () => actionRef.current?.reload()
  const columns: ProColumns<FileItem>[] = [
    {
      title: '文件名称',
      dataIndex: 'name',
      render: (_, row) => (
        <Space>
          {row.directory ? <FolderOutlined /> : <FileOutlined />}
          {row.directory && !row.symlink ? (
            <a onClick={() => setDirectory(join(directory, row.name))}>{row.name}</a>
          ) : (
            row.name
          )}
          {row.symlink && <Tag>符号链接 · 禁止操作</Tag>}
        </Space>
      ),
    },
    {
      title: '大小',
      width: 110,
      render: (_, row) => (row.directory ? '—' : `${(row.size / 1024).toFixed(1)} KB`),
    },
    { title: '更新时间', dataIndex: 'modified_at', valueType: 'dateTime', width: 180 },
    {
      title: '操作',
      valueType: 'option',
      width: 240,
      render: (_, row) =>
        row.symlink
          ? []
          : [
              !row.directory && (
                <a
                  key="read"
                  onClick={async () => {
                    const result = await api.read(site.id, join(directory, row.name))
                    setNewFile(false)
                    setEditor({ path: join(directory, row.name), content: result.content })
                  }}
                >
                  {writable ? '编辑' : '查看'}
                </a>
              ),
              !row.directory && (
                <a
                  key="download"
                  onClick={async () => {
                    const data = await api.download(site.id, join(directory, row.name))
                    const url = URL.createObjectURL(data)
                    const link = document.createElement('a')
                    link.href = url
                    link.download = row.name
                    link.click()
                    setTimeout(() => URL.revokeObjectURL(url), 1000)
                  }}
                >
                  下载
                </a>
              ),
              writable && (
                <a key="rename" onClick={() => setRenaming(row)}>
                  重命名
                </a>
              ),
              writable && (
                <Popconfirm
                  key="delete"
                  title={row.directory ? '删除这个空目录？' : '删除这个文件？'}
                  onConfirm={async () => {
                    await api.action(site.id, 'delete', join(directory, row.name))
                    message.success('已删除')
                    reload()
                  }}
                >
                  <a style={{ color: 'var(--ant-color-error)' }}>删除</a>
                </Popconfirm>
              ),
            ],
    },
  ]
  return (
    <>
      <Drawer title={`网站文件 · ${site.name}`} width="90%" open onClose={onClose}>
        <Space wrap style={{ marginBottom: 16 }}>
          <Button
            disabled={directory === '/'}
            onClick={() => setDirectory(directory.slice(0, directory.lastIndexOf('/')) || '/')}
          >
            上级目录
          </Button>
          <Button onClick={() => setDirectory('/')}>根目录</Button>
          <Tag>{directory}</Tag>
          {writable && (
            <>
              <Button
                onClick={() => {
                  setNewFile(true)
                  setEditor({ path: '', content: '' })
                }}
              >
                新建文件
              </Button>
              <Button onClick={() => setFolderOpen(true)}>新建目录</Button>
              <Upload
                showUploadList={false}
                disabled={uploading}
                beforeUpload={(file) => {
                  if (file.size > 10 * 1024 * 1024) {
                    message.error('单个文件最大 10 MiB')
                    return false
                  }
                  const send = async () => {
                    setUploading(true)
                    try {
                      await api.upload(site.id, join(directory, file.name), file)
                      message.success('上传完成')
                      reload()
                    } catch {
                      /* API displays the error */
                    } finally {
                      setUploading(false)
                    }
                  }
                  modal.confirm({
                    title: `上传 ${file.name}？`,
                    content: '同名文件将被替换。',
                    onOk: send,
                  })
                  return false
                }}
              >
                <Button icon={<UploadOutlined />} loading={uploading}>
                  上传文件
                </Button>
              </Upload>
            </>
          )}
        </Space>
        <DataTable<FileItem>
          key={directory}
          rowKey="name"
          actionRef={actionRef}
          columns={columns}
          search={false}
          pagination={false}
          request={async () => ({ data: await api.files(site.id, directory), success: true })}
        />
      </Drawer>
      {editor && (
        <DataDrawer<{ name?: string; content: string }>
          key={newFile ? 'new-file' : editor.path}
          title={newFile ? '新建文件' : editor.path}
          open
          width={900}
          onOpenChange={(open) => {
            if (!open) setEditor(null)
          }}
          initialValues={{ content: editor.content }}
          submitter={writable ? undefined : false}
          onFinish={async (values) => {
            const target = newFile ? join(directory, values.name!) : editor.path
            if (newFile) {
              const existing = await api.files(site.id, directory)
              if (existing.some((file) => file.name === values.name)) {
                message.error('文件名已存在，请通过编辑或上传替换')
                return false
              }
            }
            await api.write(site.id, target, values.content)
            message.success('已保存')
            setEditor(null)
            reload()
            return true
          }}
        >
          {newFile && <ProFormText name="name" label="文件名" rules={validName} />}
          <ProFormTextArea
            name="content"
            label="内容（UTF-8，最大 1 MiB）"
            fieldProps={{
              rows: 24,
              readOnly: !writable,
              spellCheck: false,
              style: { fontFamily: 'monospace' },
            }}
          />
        </DataDrawer>
      )}
      <DataDrawer<{ name: string }>
        title="新建目录"
        open={folderOpen}
        onOpenChange={setFolderOpen}
        onFinish={async (values) => {
          await api.action(site.id, 'mkdir', join(directory, values.name))
          message.success('已创建')
          setFolderOpen(false)
          reload()
          return true
        }}
      >
        <ProFormText name="name" label="目录名" rules={validName} />
      </DataDrawer>
      {renaming && (
        <DataDrawer<{ name: string }>
          key={renaming.name}
          title="重命名"
          open
          initialValues={{ name: renaming.name }}
          onOpenChange={(open) => {
            if (!open) setRenaming(null)
          }}
          onFinish={async (values) => {
            await api.action(
              site.id,
              'rename',
              join(directory, renaming.name),
              join(directory, values.name),
            )
            message.success('已重命名')
            setRenaming(null)
            reload()
            return true
          }}
        >
          <ProFormText name="name" label="新名称" rules={validName} />
        </DataDrawer>
      )}
    </>
  )
}
