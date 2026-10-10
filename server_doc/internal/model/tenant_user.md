# internal/model/tenant_user.go — sys_tenant_user

> 脚手架导航：[回到开发者入口](../../../README.md) · [子账号模块](../modules/tenantuser/service.md)

租户账号（主账号 + 子账号）。**账号体系放平台库**保证登录入口唯一；角色绑定放租户 schema（[user_role](user_role.md)）支持租户自定义。

| 字段 | 说明 |
| --- | --- |
| TenantID | 所属租户，索引 |
| Username | 租户内登录名，`(tenant_id, username)` 存活唯一 |
| Password | bcrypt 散列，`json:"-"` |
| Nickname | 昵称 |
| IsOwner | 是否租户主账号；主账号受保护，不可删除/停用 |
| Status | enabled/disabled |

## 跨 schema 事务

`/tenant/*` 请求的 search_path 是 `tenant_x, public`，同一事务既能写 `t_user_role` 也能写本表，因此"建账号 + 绑角色"天然原子。删除/停用/改密后需踢掉该账号全部会话。
