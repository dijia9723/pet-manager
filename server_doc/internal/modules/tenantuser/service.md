# internal/modules/tenantuser/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [handler.md](handler.md)

## 职责

租户端子账号 CRUD + 重置密码。模块名 `tenant.user`。

## 关键设计：跨库原子绑定

账号表在平台库、角色绑定表在租户 schema；请求进入 `/tenant/*` 时 search_path 已是 `tenant_x, public`，所以"建账号（sys_tenant_user）+ 绑角色（t_user_role）"在同一事务天然原子，无需跨库补偿。仓储统一用 `ctxkey.MustTenantDB(ctx)`。

## 行为要点

- `Create`：用户名租户内存活校验 → bcrypt → 写账号 → `replaceRoles` 整组替换绑定；
- `Update`：主账号受保护不可停用；改角色绑定后 `invalidatePermissions` 立即清缓存；
- `Delete`：不能删除当前登录账号（20013）；owner 受保护（20014）；软删并清绑定；
- `ResetPassword`：改密后 `revokeSessions` 踢掉该账号全部会话；
- 角色 ID 写入前经 `validRoleIDs` 校验属于本租户；
- 列表通过 `rolesOfUsers` 批量带出 role_names，避免 N+1。

## DTO

`Item`（含 role_ids/role_names）、`ListQuery`、`CreateInput`、`UpdateInput`、`ResetPasswordInput`。
