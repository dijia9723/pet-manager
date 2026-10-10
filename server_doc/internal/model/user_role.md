# internal/model/user_role.go — t_user_role

> 脚手架导航：[回到开发者入口](../../../README.md) · [tenant_user.md](tenant_user.md) · [role.md](role.md)

账号与角色的纯关联表（租户 schema），联合主键 `(user_id, role_id)`，**不继承 BaseModel**。

- `user_id` 指向平台库 `sys_tenant_user.id`，跨 schema 只存 ID、不建外键；
- "建账号 + 绑角色"在同一租户事务内完成（见 [tenantuser](../modules/tenantuser/service.md)）；
- 删除账号/角色时同步清理绑定行。
