# internal/model/role.go — t_role

> 脚手架导航：[回到开发者入口](../../../README.md) · [rbac 模块](../modules/rbac/service.md)

租户自定义角色，位于租户 schema。

| 字段 | 说明 |
| --- | --- |
| Name | 角色显示名 |
| Code | 角色编码，租户内存活唯一（`uk_t_role_code` 部分唯一索引） |
| IsSystem | 预置角色标记：true 时禁止删除、禁止改 Code |
| Remark | 备注 |

## 预置角色常量

| 常量 | 编码 | 含义 |
| --- | --- | --- |
| `RoleCodeOwner` / `RoleNameOwner` | `owner` / 主账号 | 开通租户时创建，拥有全部权限，受保护 |
| `RoleCodeStaff` / `RoleNameStaff` | `staff` / 店员 | 默认仅基础业务权限 |

与权限点的绑定见 [role_permission](role_permission.md)，与账号的绑定见 [user_role](user_role.md)。删除角色前需确认无账号引用，否则业务码 `20015`。
