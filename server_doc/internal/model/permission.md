# internal/model/permission.go — sys_permission

> 脚手架导航：[回到开发者入口](../../../README.md) · [seed](../seed/seed.md) · [菜单配置模块](../modules/permission/service.md)

权限点（菜单）字典，平台统一维护，租户角色只引用不拥有。

| 字段 | 说明 |
| --- | --- |
| ParentID | 父节点 ID，0 为根；菜单页是一棵树 |
| Code | 权限点编码（如 `tenant.user.create`），存活唯一，前后端对齐用 |
| Name | 显示名 |
| Type | `menu` 进菜单树 / `button` 按钮级 |
| Scope | `platform` / `tenant`，决定下发给哪种身份 |
| Path / Icon / Sort | 菜单路由、图标、排序 |

## 治理模式

- 初始数据由 [seed permDefs](../seed/seed.md) **只补不覆盖**；
- 名称/路径/图标/排序等改动在平台"菜单配置"页在线保存，代码不覆盖线上；
- 本表增删改后必须 `InvalidateAllPermissions` 全量失效租户权限缓存。
