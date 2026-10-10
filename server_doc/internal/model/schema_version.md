# internal/model/schema_version.go — t_schema_version

> 脚手架导航：[回到开发者入口](../../../README.md) · [migrate](../migrate/migrate.md)

租户 schema 的内部版本表，记录结构迁移执行痕迹。

| 字段 | 说明 |
| --- | --- |
| ID | 自增主键 |
| Version | 版本字符串（当前 `0001_init`，见 migrate.TenantSchemaVersion），唯一索引 |
| AppliedAt | 写入时间 autoCreateTime |

- 不继承 BaseModel、不做软删除；
- 迁移写入用 `OnConflict DoNothing`，可安全重复执行；
- 排查"某租户为什么缺表"时先查本表版本，再对照代码常量。
