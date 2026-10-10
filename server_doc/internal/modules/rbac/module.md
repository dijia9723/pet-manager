# internal/modules/rbac/module.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [service.md](service.md) · Swagger 分组「租户-角色授权」

## 职责

rbac 模块的 HTTP 层（文件名叫 module.go，因为 handler 与路由注册合并在此）。挂 `Tenant("")`，分组已内置"登录 + 租户门禁 + 租户事务"，只声明权限点。

| 方法路径 | 权限点 |
| --- | --- |
| GET `/tenant/roles` | `tenant.role.list` |
| POST `/tenant/roles` | `tenant.role.create`（body: RoleInput） |
| PUT `/tenant/roles/:id` | `tenant.role.update` |
| DELETE `/tenant/roles/:id` | `tenant.role.delete` |
| GET `/tenant/permissions` | `tenant.role.list`（授权页勾选树） |

写后读复用 `roleItem` 从列表结果中取单条，不另写组装逻辑；`:id` 由 `rolePathID` 解析。业务规则与权限解析实现见 [service.md](service.md)。
