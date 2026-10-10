# internal/modules/job/module.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [service.md](service.md) · [handler.md](handler.md)

任务模块的路由注册，命名空间 `r.Platform("/jobs")`（平台管理员身份）。

| 方法 | 路径 | 权限点 | Handler |
| --- | --- | --- | --- |
| GET | `/platform/jobs` | `platform.job.list` | handleList |
| GET | `/platform/jobs/methods` | `platform.job.list` | handleMethods（已注册方法、sh 可用性） |
| POST | `/platform/jobs` | `platform.job.create` | handleCreate |
| DELETE | `/platform/jobs/{id}` | `platform.job.delete` | handleDelete（已停用的自定义任务） |
| PUT | `/platform/jobs/{id}` | `platform.job.update` | handleUpdate（任务配置） |
| POST | `/platform/jobs/{id}/enable` | `platform.job.update` | handleEnable |
| POST | `/platform/jobs/{id}/disable` | `platform.job.update` | handleDisable |
| POST | `/platform/jobs/{id}/run` | `platform.job.trigger` | handleRun（立即执行一次） |
| GET | `/platform/jobs/{id}/logs` | `platform.job.log` | handleLogs（执行日志分页） |

模块名固定 `platform.job`，在 [bootstrap/app.md](../../bootstrap/app.md) 加入 `modules` 切片挂载；调度循环通过 `Run/Close` 与 HTTP 服务同生命周期，不走 HTTP 路由触发。
