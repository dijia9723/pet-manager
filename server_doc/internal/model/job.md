# internal/model/job.go — sys_job

> 脚手架导航：[回到开发者入口](../../../README.md) · [任务模块](../modules/job/service.md)

定时任务定义表（平台公共库），支持代码预置及页面新建的 system/http/shell 任务。

| 字段 | 列 | 说明 |
| --- | --- | --- |
| BaseModel | id/created_at/… | 见 [base.md](base.md) |
| Code | `code` | 不可修改的任务编码；存活唯一（部分唯一索引） |
| Name | `name` | 展示名称 |
| Kind | `kind` | system（系统方法）、http（HTTP/curl）、shell（sh） |
| Spec | `spec` | 5 段 cron（分 时 日 月 周），由内置解析器校验，见 [job/cron.md](../modules/job/cron.md) |
| Status | `status` | enabled/disabled，停用后 `next_run_at` 清空、不再调度 |
| Remark | `remark` | 任务说明 |
| Builtin | `builtin` | 是否代码注册；内置任务禁止删除 |
| Config | `config` | text 保存当前类型的 JSON 执行配置，系统方法键为 handler |
| TimeoutSeconds | `timeout_seconds` | 1–600 秒；新建默认 60 秒，历史/预置任务默认 600 秒 |
| LastRunAt / LastStatus / LastMessage | 同名 | 最近一次执行时间 / running·success·fail / 摘要（截断 500 字，全文看 sys_job_log） |
| NextRunAt | `next_run_at` | 下次执行时间，启用且 spec 合法时非空；扫描到期任务的判定字段 |

## 约定与坑

- 行数据"只补不覆盖"：启动/每轮 tick 由 `job.Service.syncBuiltins` 把代码注册的任务补齐，已存在（含软删）的 code 跳过；页面改的 spec/启停不会被重启回滚；
- spec 在两个时刻校验：注册内置任务时（panic，开发期暴露）与页面保存时（400）；
- 常量（JobKindSystem / JobRunRunning / JobRunSuccess / JobRunFail / JobTriggerSchedule / JobTriggerManual）定义在本文件顶部。
