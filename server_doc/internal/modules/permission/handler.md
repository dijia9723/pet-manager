# internal/modules/permission/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [service.md](service.md) · Swagger 分组「平台-菜单配置」

## 职责

权限点（菜单）字典治理 HTTP 层。

| 方法路径 | 权限点 | 入参 |
| --- | --- | --- |
| GET `/platform/permissions/tree` | `platform.permission.list` | 无，返回整棵树 |
| POST `/platform/permissions` | `platform.permission.create` | Input |
| PUT `/platform/permissions/:id` | `platform.permission.update` | Input |
| DELETE `/platform/permissions/:id` | `platform.permission.delete` | 路径 id |

`:id` 由本文件 `pathID` 解析。
