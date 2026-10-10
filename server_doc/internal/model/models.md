# internal/model/models.go — 模型登记总册

> 脚手架导航：[回到开发者入口](../../../README.md) · [migrate](../migrate/migrate.md) · [base.md](base.md)

## 职责

汇总两库模型清单供 AutoMigrate 使用。文件顺序刻意与建表顺序一致（被引用的先建）。

## PlatformModels（public schema，sys_*）

[platform_user](platform_user.md) · [tenant](tenant.md) · [tenant_user](tenant_user.md) · [permission](permission.md) · [audit_log](audit_log.md) · [config](config.md) · [dict](dict.md) · [dict_item](dict_item.md) · [login_log](login_log.md) · [job](job.md) · [job_log](job_log.md)

## TenantModels（tenant_\<id> schema，t_*）

[role](role.md) · [role_permission](role_permission.md) · [user_role](user_role.md) · [tenant_config](tenant_config.md) · [tenant_dict](tenant_dict.md) · [tenant_dict_item](tenant_dict_item.md) · [schema_version](schema_version.md)

## 新增表检查单（必做两处 + 两处配套）

1. 新建一表一文件（表名去前缀命名），模型嵌入 [BaseModel](base.md) 并写 `TableName()`；
2. **在本文件对应切片登记**，否则不会自动建表；
3. 有自然键唯一要求 → 到 [migrate](../migrate/migrate.md) 补 `WHERE deleted_at IS NULL` 的部分唯一索引；
4. 需要权限控制 → 到 [seed](../seed/seed.md) 的 `permDefs` 声明权限点。

租户新表追加后：新开租户立即带全量，存量租户在下次启动时由 `AllTenants` 批量补齐。

## 通知平台表

PlatformModels 新增 [notification](notification.md) 和 [notification_receipt](notification_receipt.md)，均在平台 schema，租户模型清单无需变化。

## 统一存储与附件

PlatformModels 追加 StorageAccount、StorageResource、StorageChange，统一平台表与 TOS，由服务强制租户归属隔离，不新增租户 schema 表。

## 网站托管表

PlatformModels 新增 ManagedServer 和 Website，分别对应 sys_managed_server/sys_website；TenantModels 无变化。网站是平台分配台账，租户按认证 TenantID 过滤查询。
