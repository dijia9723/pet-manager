# 任务管理

> [开发者入口](../../../../README.md) · [路由与权限](module.md) · [HTTP 接口](handler.md) · [Cron](cron.md) · [模型](../../model/job.md)

平台任务支持系统方法、HTTP/curl 和 sh 脚本。前端任务页可新建、编辑、启停、手动执行和查看日志；新建默认停用，可以先执行一次再启用定时调度。接口前缀为 `/api/v1/platform/jobs`。

## 新建配置

公共字段：`code`（字母开头、字母数字及 `_ . -`，最多64位，创建后不可修改）、`name`、`kind`、`spec`（5段 Cron）、`remark`（选填）、`status`（默认 disabled）、`timeout_seconds`（默认60，1–600）、`config`。

调用系统已注册方法：

```json
{
  "code": "custom_purge_schedule",
  "name": "租户到期销毁调度",
  "kind": "system",
  "spec": "30 3 * * *",
  "config": {"handler": "tenant_purge"}
}
```

方法下拉选项来自 `GET /methods`。当前注册的 `tenant_purge` 会销毁到期租户数据，按业务实际需要设置调度。

HTTP 表单配置：

```json
{
  "code": "api_sync",
  "name": "接口同步",
  "kind": "http",
  "spec": "*/15 * * * *",
  "config": {
    "url": "https://example.com/api/sync",
    "method": "POST",
    "headers": {"Content-Type": "application/json", "Authorization": "Bearer token"},
    "body": "{\"source\":\"pet-manager\"}"
  }
}
```

也可将 `config` 换为 `{"curl":"curl -X POST https://example.com/api/sync -H 'Content-Type: application/json' -d '{\"source\":\"pet-manager\"}'"}`。curl 在保存时解析为 HTTP 请求，由 Go 客户端执行，无需安装 curl。支持 `-X`、`-H`、`-d`、`--data-raw`、`--data-binary`、`--data-urlencode`、`--json`、`-u`、`-b`、`--url`、`-G`、`-I`，及常用静默/重定向选项。支持引号和反斜杠续行；不支持文件上传、输出文件、管道、命令串联或环境变量展开。复杂 curl 可改用请求表单。2xx 记成功，其余记失败，响应状态及正文记入日志；最多5次重定向。

sh 脚本配置：

```json
{
  "code": "script_sync",
  "name": "脚本同步",
  "kind": "shell",
  "spec": "0 2 * * *",
  "config": {
    "script": "#!/bin/sh\nset -eu\nprintf 'date=%s\\n' \"$1\"",
    "args": ["2026-10-09"],
    "work_dir": "/opt/pet-manager"
  }
}
```

`script` 与 `script_path` 二选一，文件路径及工作目录必须是后端服务器绝对路径。脚本使用后端进程权限和环境，参数独立传递。标准输出、错误输出、非零退出结果记录到日志，超时终止进程及其子进程。Windows 可使用 Git Bash 的 sh。

```yaml
jobs:
  shell_path: "sh" # 或 sh 可执行文件的绝对路径
```

也可通过 `PM_JOBS_SHELL_PATH` 配置。未找到 sh 时方法目录返回 `shell_available=false`，禁止保存脚本任务。请求体、脚本及 curl 各最多64 KiB；脚本参数最多64项；执行输出截断到64 KiB。

## 注册系统方法

在启动装配期调用 `RegisterMethod`，将需要开放给任务页的业务方法注册为 `TaskFunc`；不自动创建调度行。

```go
jobSvc.RegisterMethod(job.Method{
    Code: "inventory_sync",
    Name: "库存同步",
    Remark: "同步库存数据",
    Fn: inventorySvc.RunSync, // func(context.Context) (string, error)
})
```

`RegisterBuiltin(Builtin{Code, Name, Spec, Remark, Fn})` 同时注册方法并自动补预置行。方法必须在 `Run` 前注册，编码不能重复。预置行只补不覆盖，已有配置不被重启回滚。历史 system 行无 handler 时回退到原 code。

系统方法应响应 context 取消，并具备重试幂等性；Go 不能强制终止不响应取消的业务函数。

## 调度与并发

启动5秒后首次扫描，每30秒检查一次 enabled 且到期的任务，按服务器时区计算 Cron。进程内任务守卫和 Redis 锁共同限制并发：全部类型按不可修改的任务编码锁定，系统任务额外共享方法锁，多个任务调用同一方法也不会重入。运行中修改类型也无法绕过任务锁。锁30分钟并使用所有权校验释放。

手动触发先抢锁并写 running 日志，然后异步执行；配置失效或 panic 会落失败日志。手动执行不修改原调度时间。定时执行结束后按最新配置和结束时刻计算下一次时间，执行期间停用不会被完成回写重新启用。

日志含触发方式、状态、开始/结束、耗时及结果。自定义任务只能在停用且未执行时删除；软删除保留日志，原 ID 的日志接口仍可查询。预置任务禁止删除及更改名称、类型、执行方法，可修改 Cron、超时和备注。

升级后重启后端，由现有迁移同步 `config`、`timeout_seconds` 字段，并由权限种子补 `platform.job.create` / `platform.job.delete`。

## 验证

`go test ./internal/modules/job` 覆盖 Cron、curl 解析、HTTP 执行与取消、真实 sh 执行、脚本参数、非零退出、超时与子进程终止。设置 `PM_TEST_JOB_CONFIG` 为配置文件绝对路径后，额外执行 PostgreSQL/Redis 集成测试；测试仅在随机独立 schema 和 Redis 前缀中写入数据，结束后清理，使用测试方法和本地 HTTP 服务，不启动真实业务调度。
