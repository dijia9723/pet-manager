# internal/modules/dict/tenant_handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [tenant_service.md](tenant_service.md) · Swagger 分组「租户-字典管理」

## 职责

租户字典管理 HTTP 层，入参/DTO 与平台侧完全相同，共 8 个接口。

| 方法路径 | 权限点 |
| --- | --- |
| GET `/tenant/dicts` | `tenant.dict.list` |
| POST `/tenant/dicts` | `tenant.dict.create` |
| PUT `/tenant/dicts/:id` | `tenant.dict.update` |
| DELETE `/tenant/dicts/:id` | `tenant.dict.delete` |
| GET `/tenant/dicts/:id/items` | `tenant.dict.list` |
| POST `/tenant/dicts/:id/items` | `tenant.dict.create` |
| PUT `/tenant/dicts/:id/items/:itemId` | `tenant.dict.update` |
| DELETE `/tenant/dicts/:id/items/:itemId` | `tenant.dict.delete` |

路由挂 `r.Tenant("/dicts")`，天然经过租户身份门禁 + TenantScope；路径参数复用 handler.go 的 `pathID`。
