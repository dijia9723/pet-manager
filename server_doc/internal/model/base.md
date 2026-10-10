# internal/model/base.go — BaseModel

> 脚手架导航：[回到开发者入口](../../../README.md) · [models.md](models.md) · [migrate](../migrate/migrate.md)

## 职责

所有业务表统一嵌入的公共字段（平台库 `sys_*` 与租户库 `t_*` 口径一致）：

| 字段 | 列 | 说明 |
| --- | --- | --- |
| ID | `id` | uint64 自增主键 |
| CreatedAt | `created_at` | autoCreateTime（本地时区） |
| UpdatedAt | `updated_at` | autoUpdateTime |
| DeletedAt | `deleted_at` | gorm 软删除标记，带普通索引，JSON 不输出 |

## 硬约束

- 新增表**必须**嵌入 BaseModel，仅纯关联表（[role_permission](role_permission.md)、[user_role](user_role.md)）与内部版本表（[schema_version](schema_version.md)）例外；
- 软删除后自然键唯一性靠**部分唯一索引** `WHERE deleted_at IS NULL` 保证，见 [migrate](../migrate/migrate.md)；查询权限点等字典必须显式带 `deleted_at IS NULL` 条件（GORM 模型查询默认自动带，裸 SQL 需手写）。
