# internal/modules/rbac/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [权限缓存](../../infra/session/store.md)

## 职责

租户角色与授权关系的全部业务，模块名 `tenant.rbac`；同时实现 `middleware.PermissionResolver` 给鉴权链回源权限点。

## ResolvePermissions（鉴权链调用）

在租户事务内用裸 SQL 解析（后台任务场景走 TenantTx）：

- **owner 角色**：直接返回全部 `scope=tenant` 权限点——平台后续新增权限点无需回填历史租户；
- 其他账号：`t_user_role → t_role_permission → sys_permission` JOIN 去重取 code；
- 裸 SQL 不经过 GORM 软删作用域，**必须手写 `deleted_at IS NULL`**（硬约定）。

## 角色 CRUD

- `ListRoles`：从 `ctxkey.MustTenantDB` 取请求级事务，带 permission_ids 与 user_count；
- `CreateRole`：Code 租户内唯一（20016）；
- `UpdateRole`：预置角色（is_system）禁改 Code（20017）；授权整组替换后失效本租户权限缓存；
- `DeleteRole`：预置角色禁删；仍被账号引用返 20015；
- `PermissionTree`：scope=tenant 的权限点树供授权页勾选。

## DTO

`RoleInput`（name/code/remark/permission_ids）、`RoleItem`（含 user_count）、`PermissionNode`。
