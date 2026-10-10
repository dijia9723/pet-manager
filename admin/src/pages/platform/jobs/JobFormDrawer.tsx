import { Alert, Typography } from 'antd'
import {
  ProFormDependency,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components'
import { useQuery } from '@tanstack/react-query'
import { DataDrawer } from '@/components/ui'
import { jobApi } from '@/api/job'
import { useMessage } from '@/hooks/useMessage'
import type { JobCreatePayload, JobExecutionConfig, JobItem, JobKind } from '@/types'

interface JobFormValues {
  code: string
  name: string
  kind: JobKind
  spec: string
  remark: string
  timeout_seconds: number
  enable: boolean
  handler: string
  http_mode: 'form' | 'curl'
  url: string
  method: string
  headers: string
  body: string
  curl: string
  shell_source: 'inline' | 'file'
  script: string
  script_path: string
  args: string
  work_dir: string
}

const kindOptions = [
  { label: '系统方法', value: 'system' },
  { label: 'HTTP / curl 请求', value: 'http' },
  { label: 'Shell 脚本', value: 'shell' },
]

function jsonRule(type: 'headers' | 'args') {
  return {
    validator: (_: unknown, value: string) => {
      try {
        const parsed = JSON.parse(value || (type === 'headers' ? '{}' : '[]'))
        const valid =
          type === 'headers'
            ? parsed !== null &&
              typeof parsed === 'object' &&
              !Array.isArray(parsed) &&
              Object.values(parsed).every((item) => typeof item === 'string')
            : Array.isArray(parsed) && parsed.every((item) => typeof item === 'string')
        if (valid) return Promise.resolve()
      } catch {
        /* 由下方统一返回校验提示 */
      }
      return Promise.reject(
        new Error(
          type === 'headers'
            ? '请求头需为 JSON 对象，所有值都为字符串'
            : '参数需为 JSON 字符串数组，如 ["--date", "2026-10-09"]',
        ),
      )
    },
  }
}

export function JobFormDrawer({
  open,
  editing,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  editing: JobItem | null
  onOpenChange: (open: boolean) => void
  onSaved: () => void
}) {
  const message = useMessage()
  const {
    data: catalog,
    isLoading: loadingMethods,
    isError: methodsError,
  } = useQuery({
    queryKey: ['job-methods'],
    queryFn: jobApi.methods,
    enabled: open,
  })
  const builtin = Boolean(editing?.builtin)
  const config = editing?.config ?? {}
  const initialValues: Partial<JobFormValues> = {
    code: editing?.code ?? '',
    name: editing?.name ?? '',
    kind: editing?.kind ?? 'system',
    spec: editing?.spec ?? '*/15 * * * *',
    remark: editing?.remark ?? '',
    timeout_seconds: editing?.timeout_seconds ?? 60,
    enable: false,
    handler: config.handler,
    http_mode: config.curl ? 'curl' : 'form',
    url: config.url ?? '',
    method: config.method ?? 'GET',
    headers: JSON.stringify(config.headers ?? {}, null, 2),
    body: config.body ?? '',
    curl: config.curl ?? '',
    shell_source: config.script_path ? 'file' : 'inline',
    script: config.script ?? '',
    script_path: config.script_path ?? '',
    args: JSON.stringify(config.args ?? []),
    work_dir: config.work_dir ?? '',
  }

  return (
    <DataDrawer<JobFormValues>
      key={editing?.id ?? 'new-job'}
      title={editing ? `编辑任务：${editing.name}` : '新建任务'}
      width={640}
      open={open}
      onOpenChange={onOpenChange}
      initialValues={initialValues}
      onFinish={async (values) => {
        let execution: JobExecutionConfig = {}
        if (!builtin) {
          if (values.kind === 'system') execution = { handler: values.handler }
          else if (values.kind === 'http')
            execution =
              values.http_mode === 'curl'
                ? { curl: values.curl }
                : {
                    url: values.url,
                    method: values.method,
                    headers: JSON.parse(values.headers || '{}'),
                    body: values.body ?? '',
                  }
          else
            execution = {
              ...(values.shell_source === 'file'
                ? { script_path: values.script_path }
                : { script: values.script }),
              args: JSON.parse(values.args || '[]'),
              work_dir: values.work_dir ?? '',
            }
        }
        const common = {
          spec: values.spec,
          remark: values.remark ?? '',
          timeout_seconds: values.timeout_seconds,
        }
        if (editing)
          await jobApi.update(
            editing.id,
            builtin
              ? common
              : { ...common, name: values.name, kind: values.kind, config: execution },
          )
        else
          await jobApi.create({
            ...common,
            code: values.code,
            name: values.name,
            kind: values.kind,
            config: execution,
            status: values.enable ? 'enabled' : 'disabled',
          } satisfies JobCreatePayload)
        message.success(
          editing
            ? '任务已保存'
            : values.enable
              ? '任务已创建并启用'
              : '任务已创建，可立即执行或启用调度',
        )
        onSaved()
        return true
      }}
    >
      {builtin ? (
        <Typography.Paragraph type="secondary">
          系统预置任务：{config.handler || editing?.code}。可调整调度、超时和说明。
        </Typography.Paragraph>
      ) : (
        <>
          <ProFormText
            name="code"
            label="任务编码"
            disabled={!!editing}
            placeholder="如 daily_sync"
            rules={[
              { required: true, message: '请输入任务编码' },
              {
                pattern: /^[a-zA-Z][a-zA-Z0-9_.-]{0,63}$/,
                message: '以字母开头，支持字母、数字、下划线、点和连字符，最多64位',
              },
            ]}
          />
          <ProFormText
            name="name"
            label="任务名称"
            placeholder="如 每日数据同步"
            rules={[{ required: true, whitespace: true, message: '请输入任务名称' }]}
            fieldProps={{ maxLength: 128 }}
          />
          <ProFormSelect
            name="kind"
            label="任务类型"
            options={kindOptions}
            rules={[{ required: true }]}
            allowClear={false}
          />
          <ProFormDependency name={['kind', 'http_mode', 'shell_source']}>
            {({ kind, http_mode, shell_source }) =>
              kind === 'system' ? (
                <>
                  {methodsError && (
                    <Alert
                      type="error"
                      showIcon
                      message="系统方法列表加载失败，请关闭后重试"
                      style={{ marginBottom: 16 }}
                    />
                  )}
                  <ProFormSelect
                    name="handler"
                    label="执行方法"
                    options={(catalog?.methods ?? []).map((method) => ({
                      label: `${method.name}（${method.code}）`,
                      value: method.code,
                    }))}
                    fieldProps={{
                      loading: loadingMethods,
                      showSearch: true,
                      optionFilterProp: 'label',
                    }}
                    rules={[{ required: true, message: '请选择系统方法' }]}
                    extra="从系统已提供的方法中选择，新任务可使用独立的名称与调度周期。"
                  />
                  <ProFormDependency name={['handler']}>
                    {({ handler }) => (
                      <Typography.Paragraph type="secondary">
                        {catalog?.methods.find((method) => method.code === handler)?.remark}
                      </Typography.Paragraph>
                    )}
                  </ProFormDependency>
                </>
              ) : kind === 'http' ? (
                <>
                  <ProFormSelect
                    name="http_mode"
                    label="配置方式"
                    options={[
                      { label: '请求表单', value: 'form' },
                      { label: 'curl 命令', value: 'curl' },
                    ]}
                    allowClear={false}
                  />
                  {http_mode === 'curl' ? (
                    <ProFormTextArea
                      name="curl"
                      label="curl 命令"
                      placeholder={
                        'curl -X POST "https://example.com/api/sync" \\\n  -H "Content-Type: application/json" \\\n  --data-raw \'{"scope":"daily"}\''
                      }
                      rules={[{ required: true, whitespace: true, message: '请填写 curl 命令' }]}
                      fieldProps={{ rows: 7, maxLength: 65536, style: { fontFamily: 'monospace' } }}
                      extra="支持多行续写、-X、-H、-d、--data-raw、--json、-u、-b、-G。响应状态和内容会记入执行日志。"
                    />
                  ) : (
                    <>
                      <ProFormSelect
                        name="method"
                        label="请求方法"
                        options={['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'].map(
                          (value) => ({ label: value, value }),
                        )}
                        allowClear={false}
                      />
                      <ProFormText
                        name="url"
                        label="请求地址"
                        placeholder="https://example.com/api/sync"
                        rules={[
                          { required: true, message: '请输入请求地址' },
                          {
                            validator: (_, value) => {
                              try {
                                const url = new URL(value)
                                if (['http:', 'https:'].includes(url.protocol))
                                  return Promise.resolve()
                              } catch {
                                /* 统一提示 */
                              }
                              return Promise.reject(new Error('请输入有效的 HTTP/HTTPS 地址'))
                            },
                          },
                        ]}
                      />
                      <ProFormTextArea
                        name="headers"
                        label="请求头（JSON）"
                        rules={[jsonRule('headers')]}
                        fieldProps={{ rows: 3, style: { fontFamily: 'monospace' } }}
                        placeholder={'{"Content-Type":"application/json"}'}
                      />
                      <ProFormTextArea
                        name="body"
                        label="请求体"
                        fieldProps={{
                          rows: 4,
                          maxLength: 65536,
                          style: { fontFamily: 'monospace' },
                        }}
                        placeholder="选填，如 JSON 或表单内容"
                      />
                    </>
                  )}
                </>
              ) : kind === 'shell' ? (
                <>
                  {catalog && !catalog.shell_available && (
                    <Alert
                      type="warning"
                      showIcon
                      message="服务器未找到 sh，需要先配置 jobs.shell_path 才能保存脚本任务"
                      style={{ marginBottom: 16 }}
                    />
                  )}
                  <ProFormSelect
                    name="shell_source"
                    label="脚本来源"
                    options={[
                      { label: '填写脚本内容', value: 'inline' },
                      { label: '服务器 .sh 文件', value: 'file' },
                    ]}
                    allowClear={false}
                  />
                  {shell_source === 'file' ? (
                    <ProFormText
                      name="script_path"
                      label="脚本绝对路径"
                      placeholder="/opt/pet-manager/scripts/sync.sh"
                      rules={[{ required: true, message: '请输入服务器上的脚本路径' }]}
                    />
                  ) : (
                    <ProFormTextArea
                      name="script"
                      label="sh 脚本内容"
                      placeholder={'#!/bin/sh\nset -eu\nprintf "任务执行完成\\n"'}
                      rules={[{ required: true, whitespace: true, message: '请填写脚本内容' }]}
                      fieldProps={{ rows: 8, maxLength: 65536, style: { fontFamily: 'monospace' } }}
                    />
                  )}
                  <ProFormTextArea
                    name="args"
                    label="脚本参数（JSON 数组）"
                    rules={[jsonRule('args')]}
                    fieldProps={{ rows: 2 }}
                    placeholder={'["--date", "2026-10-09"]'}
                  />
                  <ProFormText
                    name="work_dir"
                    label="工作目录"
                    placeholder="选填，服务器上的绝对路径"
                    extra="脚本在后端服务器执行，标准输出、错误输出和退出结果会记入执行日志。"
                  />
                </>
              ) : null
            }
          </ProFormDependency>
        </>
      )}
      <ProFormText
        name="spec"
        label="Cron 表达式"
        placeholder="如 30 3 * * *"
        rules={[
          { required: true, message: '请输入 Cron 表达式' },
          {
            validator: (_, value) =>
              value?.trim().split(/\s+/).length === 5
                ? Promise.resolve()
                : Promise.reject(new Error('需要5段：分 时 日 月 周')),
          },
        ]}
        extra="5 段：分 时 日 月 周；如 */15 * * * * 表示每15分钟，30 3 * * * 表示每天03:30。按服务器时区调度。"
      />
      <ProFormDigit
        name="timeout_seconds"
        label="执行超时（秒）"
        rules={[{ required: true, message: '请输入执行超时' }]}
        fieldProps={{ min: 1, max: 600, precision: 0 }}
      />
      <ProFormTextArea
        name="remark"
        label="任务说明"
        fieldProps={{ rows: 2, maxLength: 255, showCount: true }}
      />
      {!editing && (
        <ProFormSwitch
          name="enable"
          label="保存后启用定时调度"
          extra="默认停用；保存后仍可立即执行一次。"
        />
      )}
    </DataDrawer>
  )
}
