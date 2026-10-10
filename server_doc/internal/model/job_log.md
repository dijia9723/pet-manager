# internal/model/job_log.go — sys_job_log

> 脚手架导航：[回到开发者入口](../../../README.md) · [任务模块](../modules/job/service.md)

定时任务执行日志表（平台公共库），每次调度触发或手动执行先落一条 `running`，结束回写结果。

| 字段 | 列 | 说明 |
| --- | --- | --- |
| BaseModel | id/created_at/… | 见 [base.md](base.md) |
| JobID | `job_id` | 关联 sys_job.id（逻辑关联，不建外键），复合索引 `(job_id, started_at DESC)` |
| JobCode | `job_code` | 冗余任务编码，任务行被删后日志仍可读 |
| Trigger | `trigger` | `schedule` 调度触发 / `manual` 页面手动执行 |
| Status | `status` | `running` / `success` / `fail`，普通索引 |
| StartedAt / FinishedAt | 同名 | 开始时间（索引）/ 结束时间，running 中为空 |
| CostMs | `cost_ms` | 执行耗时毫秒 |
| Result | `result` | text：成功为执行摘要，失败为错误信息（可能含摘要前缀） |

## 约定

- 平台侧只提供分页只读接口 `GET /platform/jobs/{id}/logs`，不提供修改/删除；
- 日志随任务执行持续增长，当前脚手架不做自动清理；后续需要时优先新增一个内置清理任务而不是在业务代码里删。
