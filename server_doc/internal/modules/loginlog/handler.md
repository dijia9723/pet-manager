# internal/modules/loginlog/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [service.md](service.md) · Swagger 分组「平台-登录日志」

## 职责

登录日志只读查询 HTTP 层，唯一接口：

| 方法路径 | 权限点 | 入参 |
| --- | --- | --- |
| GET `/platform/login-logs` | `platform.loginlog.list` | ListQuery（username/status/tenant 等 + 分页） |

写入不经过 HTTP，由 auth 模块在登录流程中调用 Service 的记录方法，成功失败都采集。
