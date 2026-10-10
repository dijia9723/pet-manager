# internal/modules/tenantuser/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [service.md](service.md) · Swagger 分组「租户-子账号管理」

## 职责

租户端子账号管理 HTTP 层（路由注册在 [service.md](service.md) 的 `Register`，挂 `Tenant("/users")`）。

| 方法路径 | 权限点 | 入参 |
| --- | --- | --- |
| GET `/tenant/users` | `tenant.user.list` | ListQuery（keyword 分页） |
| POST `/tenant/users` | `tenant.user.create` | CreateInput |
| PUT `/tenant/users/:id` | `tenant.user.update` | UpdateInput |
| DELETE `/tenant/users/:id` | `tenant.user.delete` | 路径 id |
| POST `/tenant/users/:id/reset-password` | `tenant.user.reset` | ResetPasswordInput |

`:id` 由 `userPathID` 解析。所有 handler 模式一致：绑参 → 调 Service → response.OK/Fail。
