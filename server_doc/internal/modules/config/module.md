# internal/modules/config/module.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [handler.md](handler.md) · [service.md](service.md) · [tenant_handler.md](tenant_handler.md) · [tenant_service.md](tenant_service.md)

## 职责

参数配置模块路由注册，模块名 `config`（同时承载平台侧与租户侧）。

| 分组 | 数据表 | 路径 | 权限点前缀 |
| --- | --- | --- | --- |
| `Platform("/configs")` | sys_config | `/platform/configs` | `platform.param.*` |
| `Tenant("/configs")` | t_config | `/tenant/configs` | `tenant.param.*` |

两侧各 4 个标准 CRUD 接口。租户分组天然经过租户身份门禁 + TenantScope。

注意 import 时包名在 [bootstrap](../../bootstrap/app.md) 中以 `sysconfig` 别名引入，避免与 `internal/config` 配置包混淆。
