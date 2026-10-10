# internal/modules/permission/module.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [handler.md](handler.md) · [service.md](service.md)

## 职责

菜单配置模块路由注册，模块名 `platform.permission`。挂 `Platform("/permissions")`，提供树查询 + 增/改/删四个接口，权限点为 `platform.permission.list/create/update/delete`。

注意列表是 `/tree` 而非根路径；POST 仍打在集合根 `/permissions` 上。
