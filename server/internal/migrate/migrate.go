// Package migrate 负责平台库与租户 schema 的结构迁移。
//
// 为什么用 AutoMigrate：
//   - 平台库与租户库的模型都在代码里定义，模型本身就是版本化的模板；
//   - 租户表在 W2–W5 会持续新增（宠物档案 / 工单 / 打印模板…），
//     用 AutoMigrate 可让"新开租户立即带全表、存量租户批量补齐"成为一行调用；
//   - 每次迁移后写入 t_schema_version，保留"按租户批量升级"的可追溯性。
package migrate

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
)

// TenantSchemaVersion 当前租户 schema 的结构版本号。
// 新增租户业务表时同步递增，便于排查"某租户表为什么少了"的问题。
const TenantSchemaVersion = "0002_tenant_config_dict"

// Platform 迁移平台公共库。
func Platform(ctx context.Context, m *db.Manager) error {
	tx := m.Platform().WithContext(ctx)
	// 1) 按模型建表：AutoMigrate 只增不删，缺表补建、缺列补列。
	if err := tx.AutoMigrate(model.PlatformModels()...); err != nil {
		return errcode.Wrap(errcode.ErrDatabase, fmt.Errorf("迁移平台库失败: %w", err))
	}
	// 2) 补齐"部分唯一索引"，约束软删除后的存活行。
	for _, stmt := range platformPartialUniqueIndexes {
		if err := tx.Exec(stmt).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, fmt.Errorf("创建平台库唯一索引失败: %w", err))
		}
	}
	// 3) 补齐 AutoMigrate 无法表达的普通/复合索引。
	for _, stmt := range platformIndexes {
		if err := tx.Exec(stmt).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, fmt.Errorf("创建平台库索引失败: %w", err))
		}
	}
	// 4) 幂等清理已下线模块遗留的表与列。
	for _, stmt := range platformCleanup {
		if err := tx.Exec(stmt).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, fmt.Errorf("清理平台库遗留结构失败: %w", err))
		}
	}
	return nil
}

// TenantSchema 创建并初始化一个租户的 schema，可重复执行（幂等）。
func TenantSchema(ctx context.Context, m *db.Manager, schema string) error {
	if err := db.ValidateSchema(schema); err != nil {
		return err
	}
	if err := m.CreateTenantSchema(ctx, schema); err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	return m.TenantTx(ctx, schema, func(tx *gorm.DB) error {
		// 建租户业务表：新开租户即带全表，存量租户由 AllTenants 批量补齐。
		if err := tx.AutoMigrate(model.TenantModels()...); err != nil {
			return errcode.Wrap(errcode.ErrDatabase, fmt.Errorf("迁移租户 schema %s 失败: %w", schema, err))
		}
		// 补齐租户侧部分唯一索引。
		for _, stmt := range tenantPartialUniqueIndexes {
			if err := tx.Exec(stmt).Error; err != nil {
				return errcode.Wrap(errcode.ErrDatabase, fmt.Errorf("创建租户唯一索引失败: %w", err))
			}
		}
		// 写入当前结构版本用于审计；OnConflict DoNothing 保证重复执行不报错、幂等。
		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&model.SchemaVersion{Version: TenantSchemaVersion}).Error
	})
}

// platformPartialUniqueIndexes / tenantPartialUniqueIndexes 是"部分唯一索引"。
//
// 因为 BaseModel 带软删除（deleted_at），如果直接用普通唯一索引，
// 被删除的历史行会永久占用用户名、租户编码这类业务键，导致"删了却重建不了"。
// 加上 WHERE deleted_at IS NULL 后，唯一性只约束存活行，历史行不参与。
// PostgreSQL 支持这类部分索引，这里在 AutoMigrate 之后统一补齐。
var platformPartialUniqueIndexes = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_website_cms_port ON sys_website (server_id, cms_port) WHERE deleted_at IS NULL AND cms_port > 0`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_storage_account_tenant ON sys_storage_account (tenant_id) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_storage_resource_key ON sys_storage_resource (bucket, object_key) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_notification_receipt_recipient ON sys_notification_receipt (notification_id, user_type, tenant_id, user_id) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_platform_user_username ON sys_platform_user (username) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_tenant_code ON sys_tenant (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_tenant_schema ON sys_tenant (schema_name) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_tenant_user_username ON sys_tenant_user (tenant_id, username) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_permission_code ON sys_permission (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_config_key ON sys_config (config_key) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_dict_code ON sys_dict (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_dict_item_value ON sys_dict_item (dict_id, value) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_job_code ON sys_job (code) WHERE deleted_at IS NULL`,
}

var tenantPartialUniqueIndexes = []string{
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_t_role_code ON t_role (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_t_config_key ON t_config (config_key) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_t_dict_code ON t_dict (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_t_dict_item_value ON t_dict_item (dict_id, value) WHERE deleted_at IS NULL`,
}

// platformIndexes 是非唯一的普通索引，用于 AutoMigrate 无法表达的场景
// （如涉及 BaseModel.created_at 的复合索引）。
// sys_audit_log / sys_login_log 增长快，按"租户+时间""操作人+时间"两个查询维度建索引。
var platformIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_sys_audit_log_tenant_time ON sys_audit_log (tenant_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_sys_audit_log_user_time ON sys_audit_log (user_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_sys_login_log_tenant_time ON sys_login_log (tenant_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_sys_login_log_username_time ON sys_login_log (username, created_at DESC)`,
	// 租户销毁任务按 status+purge_at 扫描到期行；任务日志按任务+开始时间倒序展示
	`CREATE INDEX IF NOT EXISTS idx_sys_tenant_purge ON sys_tenant (status, purge_at)`,
	`CREATE INDEX IF NOT EXISTS idx_sys_job_log_job_started ON sys_job_log (job_id, started_at DESC)`,
}

// platformCleanup 回收已下线模块遗留的表与列。
// AutoMigrate 只增不删，模型移除后旧表/旧列不会自动消失，这里显式清理（幂等）。
var platformCleanup = []string{
	`DROP TABLE IF EXISTS sys_tenant_package`,
	`ALTER TABLE sys_tenant DROP COLUMN IF EXISTS package_id`,
	`ALTER TABLE sys_tenant DROP COLUMN IF EXISTS features`,
}

// AllTenants 遍历存量租户补齐表结构，用于发版后批量升级。
func AllTenants(ctx context.Context, m *db.Manager, log *zap.Logger) error {
	var tenants []model.Tenant
	if err := m.Platform().WithContext(ctx).Find(&tenants).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	for _, t := range tenants {
		if err := TenantSchema(ctx, m, t.SchemaName); err != nil {
			log.Error("升级租户 schema 失败", zap.Uint64("tenant_id", t.ID), zap.String("schema", t.SchemaName), zap.Error(err))
			return err
		}
		log.Info("租户 schema 已升级", zap.Uint64("tenant_id", t.ID), zap.String("schema", t.SchemaName))
	}
	return nil
}
