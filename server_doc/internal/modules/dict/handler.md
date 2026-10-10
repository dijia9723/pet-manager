# internal/modules/dict/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [service.md](service.md) · [tenant_handler.md](tenant_handler.md) · Swagger 分组「平台-字典管理」

## 职责

字典类型 + 字典项两级资源的 HTTP 层。

字典类型（挂 `/platform/dicts`）：

| 方法路径 | 权限点 | 入参 |
| --- | --- | --- |
| GET `` | `platform.dict.list` | ListQuery 分页 |
| POST `` | `platform.dict.create` | DictInput |
| PUT `/:id` | `platform.dict.update` | DictInput |
| DELETE `/:id` | `platform.dict.delete` | 路径 id |

字典项（嵌套资源）：

| 方法路径 | 权限点 | 入参 |
| --- | --- | --- |
| GET `/:id/items` | `platform.dict.list` | 字典 id |
| POST `/:id/items` | `platform.dict.create` | DictItemInput |
| PUT `/:id/items/:itemId` | `platform.dict.update` | DictItemInput |
| DELETE `/:id/items/:itemId` | `platform.dict.delete` | 两个路径参数 |

路径参数统一经 `pathID(c, name)` 解析。
