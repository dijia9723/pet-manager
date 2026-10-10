# internal/modules/config/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [service.md](service.md) · [tenant_handler.md](tenant_handler.md) · Swagger 分组「平台-参数配置」

## 职责

平台参数配置 HTTP 层。

| 方法路径 | 权限点 | 入参 |
| --- | --- | --- |
| GET `/platform/configs` | `platform.param.list` | ListQuery（keyword/group_key/status + 分页） |
| POST `/platform/configs` | `platform.param.create` | Input |
| PUT `/platform/configs/:id` | `platform.param.update` | Input |
| DELETE `/platform/configs/:id` | `platform.param.delete` | 路径 id |

`:id` 由本文件 `pathID` 解析。
