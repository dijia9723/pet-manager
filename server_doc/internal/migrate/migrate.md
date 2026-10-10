# internal/migrate/migrate.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [model 总览](../model/models.md) · [db.Manager](../infra/db/manager.md)

## 职责

平台库与租户 schema 的结构迁移，启动时由 bootstrap 自动执行。

## 为什么用 AutoMigrate

- 模型即版本化模板，模型改了结构自动补齐；
- 新开租户立即带全量表；存量租户靠 `AllTenants` 一行调用批量补齐；
- 每个租户 schema 写 `t_schema_version`（当前 `0002_tenant_config_dict`），新增租户表时同步递增该常量。

## 三个入口

| 函数 | 时机 | 内容 |
| --- | --- | --- |
| `Platform(ctx, m)` | 每次启动 | 平台表 AutoMigrate + 部分唯一索引 + 普通索引 + `platformCleanup` 遗留清理（均幂等） |
| `TenantSchema(ctx, m, schema)` | 开通租户 + 启动补齐 | 建 schema、租户表 AutoMigrate、部分唯一索引、写版本（OnConflict DoNothing） |
| `AllTenants(ctx, m, log)` | 每次启动 | 遍历全部租户逐个 `TenantSchema`，失败即中止启动 |

## 部分唯一索引（重要约定）

BaseModel 带软删除，普通唯一索引会让被删历史行永久占用业务键。所有自然键唯一性都用：

```sql
CREATE UNIQUE INDEX IF NOT EXISTS uk_xxx ON table (key) WHERE deleted_at IS NULL
```

新增有自然键的表时，必须到本文件对应切片补一条。AutoMigrate **只增不删**，废弃表/列在 `platformCleanup` 里显式幂等 DROP。

## 任务调度与租户销毁相关索引（platformIndexes / platformPartialUniqueIndexes）

- `uk_sys_job_code ON sys_job (code) WHERE deleted_at IS NULL`：任务编码存活唯一；
- `idx_sys_tenant_purge ON sys_tenant (status, purge_at)`：到期销毁扫描 `status='deleted' AND purge_at <= now()`；
- `idx_sys_job_log_job_started ON sys_job_log (job_id, started_at DESC)`：执行日志按任务倒序分页；`status` 另有单列普通索引。

## 通知回执唯一索引

Platform AutoMigrate 增加 sys_notification 和 sys_notification_receipt。uk_notification_receipt_recipient 对存活记录的 notification_id、user_type、tenant_id、user_id 施加唯一约束，防止重复投递。不新增租户表，无需递增 TenantSchemaVersion。

## 统一存储与附件

存储迁移新增三张 sys_storage_* 表、DictItem.StorageBytes 和 Notification.Attachments。部分唯一索引 uk_storage_account_tenant（tenant_id）及 uk_storage_resource_key（bucket,object_key）。全部平台 schema，TenantSchemaVersion 无需递增。

## CMS 端口

新增 sys_website 的 (server_id, cms_port) 部分唯一索引，条件 deleted_at IS NULL AND cms_port > 0。数据库预留端口避免并发分配冲突；零值为尚未部署。
