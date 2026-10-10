# internal/modules/config/tenant_handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [tenant_service.md](tenant_service.md) · Swagger 分组「租户-参数配置」

## 职责

租户参数配置 HTTP 层，入参/DTO 与平台侧完全相同。

| 方法路径 | 权限点 | 入参 |
| --- | --- | --- |
| GET `/tenant/configs` | `tenant.param.list` | ListQuery（keyword/group_key/status + 分页） |
| POST `/tenant/configs` | `tenant.param.create` | Input |
| PUT `/tenant/configs/:id` | `tenant.param.update` | Input |
| DELETE `/tenant/configs/:id` | `tenant.param.delete` | 路径 id |

路由挂 `r.Tenant("/configs")`，天然经过租户身份门禁 + TenantScope；`:id` 复用 handler.go 的 `pathID`。
