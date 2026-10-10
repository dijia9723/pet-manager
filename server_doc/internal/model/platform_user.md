# internal/model/platform_user.go — sys_platform_user

> 脚手架导航：[回到开发者入口](../../../README.md) · [seed](../seed/seed.md)

平台侧管理员账号，仅平台运维使用，从无租户编码的登录入口登录。

| 字段 | 说明 |
| --- | --- |
| Username | 登录名，存活唯一（部分唯一索引 `uk_sys_platform_user_username`） |
| Password | bcrypt 散列，`json:"-"` 永不输出 |
| Nickname | 昵称 |
| Status | enabled/disabled |

- 首个管理员由 [seed](../seed/seed.md) 创建，口令生产环境用 `PM_SEED_PLATFORM_ADMIN_PASSWORD` 注入；
- 平台管理员在 RBAC 中按通配 `*` 授权，不走角色表；
- 与 [tenant_user](tenant_user.md) 是两套账号表，但登录入口唯一（按 tenant_code 有无分流）。
