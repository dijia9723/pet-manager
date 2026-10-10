import { useEffect, useRef, useState } from 'react'
import { PageContainer } from '@/components/PageContainer'
import { Drawer, Popconfirm, Switch, Table, Tag, Tooltip, Typography } from 'antd'
import { InfoCircleOutlined, PlusOutlined } from '@ant-design/icons'
import dayjs from 'dayjs'
import { jobApi } from '@/api/job'
import { PermButton } from '@/components/PermButton'
import { DataTable, type ActionType, type ProColumns } from '@/components/ui'
import { useMessage } from '@/hooks/useMessage'
import { usePermission } from '@/hooks/usePermission'
import type { JobItem, JobKind, JobLogItem, JobRunStatus } from '@/types'
import { JobFormDrawer } from './JobFormDrawer'

/** 耗时格式化：毫秒 → 秒 */
function formatCost(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)} s`
  return `${ms} ms`
}

const runStatusMeta: Record<JobRunStatus, { color: string; text: string }> = {
  running: { color: 'processing', text: '执行中' },
  success: { color: 'success', text: '成功' },
  fail: { color: 'error', text: '失败' },
}

const kindMeta: Record<JobKind, { color: string; text: string }> = {
  system: { color: 'blue', text: '系统方法' },
  http: { color: 'cyan', text: 'HTTP / curl' },
  shell: { color: 'purple', text: 'Shell 脚本' },
}

export default function JobListPage() {
  const actionRef = useRef<ActionType>(null)
  const message = useMessage()
  const { has } = usePermission()

  const [editing, setEditing] = useState<JobItem | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [logJob, setLogJob] = useState<JobItem | null>(null)

  const columns: ProColumns<JobItem>[] = [
    {
      title: '任务名称',
      dataIndex: 'name',
      search: false,
      render: (_, record) => (
        <div>
          <div>{record.name}</div>
          <div style={{ fontSize: 12, color: 'var(--ant-color-text-secondary)', marginTop: 4 }}>
            {record.code}
            {record.builtin && <Tag style={{ marginLeft: 8 }}>系统预置</Tag>}
          </div>
        </div>
      ),
    },
    {
      title: '类型',
      dataIndex: 'kind',
      search: false,
      width: 130,
      render: (_, record) => (
        <Tag color={kindMeta[record.kind]?.color}>{kindMeta[record.kind]?.text ?? record.kind}</Tag>
      ),
    },
    {
      title: '说明',
      dataIndex: 'remark',
      search: false,
      ellipsis: true,
      render: (_, record) => (
        <Tooltip title={record.remark}>
          <span style={{ color: 'var(--ant-color-text-secondary)' }}>{record.remark || '-'}</span>
        </Tooltip>
      ),
    },
    {
      title: (
        <span>
          Cron 表达式{' '}
          <Tooltip title="5 段格式：分 时 日 月 周。示例：30 3 * * *（每日 03:30）、*/15 * * * *（每 15 分钟）、0 9 * * 1-5（工作日 09:00）">
            <InfoCircleOutlined style={{ color: 'var(--ant-color-text-tertiary)' }} />
          </Tooltip>
        </span>
      ),
      dataIndex: 'spec',
      search: false,
      width: 180,
      render: (_, record) => <Typography.Text code>{record.spec}</Typography.Text>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      search: false,
      width: 90,
      render: (_, record) => (
        <Switch
          size="small"
          checked={record.status === 'enabled'}
          checkedChildren="启用"
          unCheckedChildren="停用"
          disabled={!has('platform.job.update')}
          onChange={async (checked) => {
            if (checked) {
              await jobApi.enable(record.id)
              message.success('任务已启用')
            } else {
              await jobApi.disable(record.id)
              message.success('任务已停用')
            }
            actionRef.current?.reload()
          }}
        />
      ),
    },
    {
      title: '上次执行',
      dataIndex: 'last_run_at',
      search: false,
      width: 200,
      render: (_, record) =>
        record.last_run_at ? (
          <div>
            <Tooltip title={record.last_message}>
              <span>{dayjs(record.last_run_at).format('YYYY-MM-DD HH:mm:ss')}</span>
            </Tooltip>
            {record.last_status && (
              <div style={{ marginTop: 2 }}>
                <Tag color={runStatusMeta[record.last_status as JobRunStatus]?.color ?? 'default'}>
                  {runStatusMeta[record.last_status as JobRunStatus]?.text ?? record.last_status}
                </Tag>
              </div>
            )}
          </div>
        ) : (
          <span style={{ color: 'var(--ant-color-text-tertiary)' }}>从未执行</span>
        ),
    },
    {
      title: '下次执行',
      dataIndex: 'next_run_at',
      search: false,
      width: 170,
      render: (_, record) =>
        record.status === 'enabled' && record.next_run_at ? (
          dayjs(record.next_run_at).format('YYYY-MM-DD HH:mm')
        ) : (
          <span style={{ color: 'var(--ant-color-text-tertiary)' }}>-</span>
        ),
    },
    {
      title: '操作',
      valueType: 'option',
      key: 'option',
      width: 260,
      render: (_, record) => [
        <PermButton
          key="edit"
          perm="platform.job.update"
          type="link"
          size="small"
          onClick={() => {
            setEditing(record)
            setEditOpen(true)
          }}
        >
          编辑
        </PermButton>,
        !record.builtin && has('platform.job.delete') && (
          <Popconfirm
            key="delete"
            title={`删除任务「${record.name}」？`}
            description="仅删除任务配置，历史执行日志保留。"
            disabled={record.status !== 'disabled' || record.last_status === 'running'}
            onConfirm={async () => {
              await jobApi.remove(record.id)
              message.success('任务已删除')
              actionRef.current?.reload()
            }}
          >
            <PermButton
              perm="platform.job.delete"
              type="link"
              size="small"
              danger
              disabled={record.status !== 'disabled' || record.last_status === 'running'}
              title={
                record.status !== 'disabled'
                  ? '请先停用任务'
                  : record.last_status === 'running'
                    ? '执行中的任务不能删除'
                    : '删除任务'
              }
            >
              删除
            </PermButton>
          </Popconfirm>
        ),
        <Popconfirm
          key="run"
          title={`立即执行「${record.name}」？`}
          description="异步执行，不影响原有调度，结果可在执行日志查看"
          okText="执行"
          cancelText="取消"
          onConfirm={async () => {
            await jobApi.run(record.id)
            message.success('已触发执行，请稍后查看执行日志')
            actionRef.current?.reload()
          }}
        >
          <PermButton perm="platform.job.trigger" type="link" size="small">
            立即执行
          </PermButton>
        </Popconfirm>,
        <PermButton
          key="logs"
          perm="platform.job.log"
          type="link"
          size="small"
          onClick={() => setLogJob(record)}
        >
          执行日志
        </PermButton>,
      ],
    },
  ]

  return (
    <PageContainer title="任务管理">
      <DataTable<JobItem>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        pagination={false}
        options={{ density: false, setting: false, reload: true }}
        polling={5000}
        scroll={{ x: 1350 }}
        toolBarRender={() => [
          <PermButton
            key="create"
            perm="platform.job.create"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null)
              setEditOpen(true)
            }}
          >
            新建任务
          </PermButton>,
        ]}
        request={async () => {
          const res = await jobApi.list()
          return { data: res ?? [], success: true }
        }}
      />

      <JobFormDrawer
        open={editOpen}
        editing={editing}
        onOpenChange={setEditOpen}
        onSaved={() => actionRef.current?.reload()}
      />

      <JobLogDrawer job={logJob} onClose={() => setLogJob(null)} />
    </PageContainer>
  )
}

/** 任务执行日志抽屉：服务端分页的执行历史 */
function JobLogDrawer({ job, onClose }: { job: JobItem | null; onClose: () => void }) {
  const [data, setData] = useState<JobLogItem[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [loading, setLoading] = useState(false)
  // 切换任务时在 render 阶段重置分页（React 官方推荐的"渲染时调整 state"模式）
  const [trackedId, setTrackedId] = useState<number | null>(null)
  if (job && job.id !== trackedId) {
    setTrackedId(job.id)
    setPage(1)
    setPageSize(10)
    setLoading(true)
  }

  useEffect(() => {
    if (!job) return
    let cancelled = false
    const load = () =>
      jobApi
        .logs(job.id, { page, page_size: pageSize })
        .then((res) => {
          if (cancelled) return
          setData(res?.list ?? [])
          setTotal(res?.total ?? 0)
        })
        .catch(() => {
          if (!cancelled) {
            setData([])
            setTotal(0)
          }
        })
        .finally(() => {
          if (!cancelled) setLoading(false)
        })
    void load()
    const timer = window.setInterval(() => void load(), 3000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [job, page, pageSize])

  return (
    <Drawer
      title={job ? `执行日志：${job.name}` : '执行日志'}
      width={720}
      open={!!job}
      onClose={onClose}
      destroyOnHidden
    >
      <Table<JobLogItem>
        rowKey="id"
        size="middle"
        loading={loading}
        dataSource={data}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          onChange: (p, ps) => {
            setPage(p)
            setPageSize(ps)
          },
        }}
        expandable={{
          expandedRowRender: (record) => (
            <Typography.Paragraph style={{ marginBottom: 0, whiteSpace: 'pre-wrap' }}>
              {record.result || '-'}
            </Typography.Paragraph>
          ),
          rowExpandable: (record) => !!record.result,
        }}
        columns={[
          {
            title: '状态',
            dataIndex: 'status',
            width: 90,
            render: (status: JobRunStatus) => (
              <Tag color={runStatusMeta[status]?.color ?? 'default'}>
                {runStatusMeta[status]?.text ?? status}
              </Tag>
            ),
          },
          {
            title: '触发方式',
            dataIndex: 'trigger',
            width: 90,
            render: (v: string) =>
              v === 'manual' ? <Tag color="purple">手动</Tag> : <Tag color="cyan">调度</Tag>,
          },
          {
            title: '开始时间',
            dataIndex: 'started_at',
            width: 170,
            render: (v: string) => dayjs(v).format('YYYY-MM-DD HH:mm:ss'),
          },
          {
            title: '耗时',
            dataIndex: 'cost_ms',
            width: 90,
            render: (v: number) => (v ? formatCost(v) : '-'),
          },
          {
            title: '执行结果',
            dataIndex: 'result',
            ellipsis: true,
            render: (v: string) => (
              <Typography.Paragraph
                ellipsis={{ rows: 1, tooltip: v }}
                style={{ marginBottom: 0, color: 'var(--ant-color-text-secondary)' }}
              >
                {v || '-'}
              </Typography.Paragraph>
            ),
          },
        ]}
      />
    </Drawer>
  )
}
