# internal/modules/dict/module.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [handler.md](handler.md) · [service.md](service.md) · [tenant_handler.md](tenant_handler.md) · [tenant_service.md](tenant_service.md)

## 职责

字典模块路由注册，模块名 `dict`（同时承载平台侧与租户侧）。两侧各 8 个接口，覆盖字典类型 CRUD 与嵌套字典项 CRUD。

| 分组 | 数据表 | 路径 | 权限点前缀 |
| --- | --- | --- | --- |
| `Platform("/dicts")` | sys_dict / sys_dict_item | `/platform/dicts` | `platform.dict.*` |
| `Tenant("/dicts")` | t_dict / t_dict_item | `/tenant/dicts` | `tenant.dict.*` |

租户分组天然经过租户身份门禁 + TenantScope。
