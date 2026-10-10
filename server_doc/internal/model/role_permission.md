# internal/model/role_permission.go — t_role_permission

> 脚手架导航：[回到开发者入口](../../../README.md) · [role.md](role.md) · [permission.md](permission.md)

角色与权限点的纯关联表（租户 schema），联合主键 `(role_id, permission_id)`，**不继承 BaseModel**、不软删除，授权整组替换。

- `permission_id` 指向平台库 `sys_permission.id`，跨 schema 只存 ID 不建外键，保证租户 schema 未来可独立搬迁；
- 因为 search_path 带 `public`，租户事务里可以 JOIN 平台权限点字典；
- 替换授权后需失效该租户的权限缓存。
