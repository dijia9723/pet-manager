// Package db 是多租户数据访问的唯一入口。
//
// 设计要点（对应 prd.md 模块 1 / plan.md 关键技术决策）：
//
//  1. 只有一个连接池。平台库查询直接用 Platform()；
//     租户业务数据必须走 TenantTx / BeginTenant，在事务内用
//     set_config('search_path', 'tenant_x, public', true) 切换 schema。
//     is_local=true 等价于 SET LOCAL，事务提交/回滚后连接自动复位，
//     因此连接池复用不会串租户。
//  2. schema 名由系统按 tenant_<id> 生成并做正则校验，不接受用户输入，无注入面。
//  3. 租户域代码禁止直接使用 Platform()，这是代码评审的硬性检查点。
package db

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"pet-manager/server/internal/config"
	"pet-manager/server/internal/pkg/errcode"
)

// tenantSchemaPattern 限定 schema 名形态，既保证安全也统一命名。
var tenantSchemaPattern = regexp.MustCompile(`^tenant_[0-9]+$`)

// Manager 持有唯一的连接池，并负责租户事务的开启/结束。
type Manager struct {
	pool           *gorm.DB    // 全局唯一连接池
	platformSchema string      // 平台公共库 schema 名
	log            *zap.Logger // 回滚等异常日志
}

// New 建立连接池并应用连接数、生命周期等参数。
func New(cfg config.DatabaseConfig, log *zap.Logger) (*Manager, error) {
	gdb, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                 gormlogger.Default.LogMode(parseLogLevel(cfg.LogLevel)),
		SkipDefaultTransaction: true, // 事务由业务显式控制，避免隐式事务与租户事务嵌套
		NowFunc:                func() time.Time { return time.Now().In(time.Local) },
	})
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接池失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return &Manager{pool: gdb, platformSchema: cfg.PlatformSchema, log: log}, nil
}

// Platform 返回平台公共库句柄。租户域代码不得调用。
func (m *Manager) Platform() *gorm.DB { return m.pool }

// PlatformSchema 返回平台公共库 schema 名。
func (m *Manager) PlatformSchema() string { return m.platformSchema }

// Close 关闭底层连接池，供进程退出时调用。
func (m *Manager) Close() error {
	sqlDB, err := m.pool.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// ValidateSchema 校验租户 schema 名是否合法。
func ValidateSchema(schema string) error {
	if !tenantSchemaPattern.MatchString(schema) {
		return errcode.Newf(errcode.ErrTenantSchemaBad, "非法的租户数据空间: %q", schema)
	}
	return nil
}

// SchemaForTenant 由租户 ID 推导 schema 名，全项目唯一实现。
func SchemaForTenant(tenantID uint64) string {
	return fmt.Sprintf("tenant_%d", tenantID)
}

// TenantTx 在租户 schema 内执行一段逻辑，是租户数据的强制入口。
// 事务结束（提交或回滚）后连接自动复位，不会污染连接池中的其他请求。
func (m *Manager) TenantTx(ctx context.Context, schema string, fn func(tx *gorm.DB) error) error {
	tx, err := m.BeginTenant(ctx, schema)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			_ = m.EndTenant(tx, false)
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		_ = m.EndTenant(tx, false)
		return err
	}
	return m.EndTenant(tx, true)
}

// BeginTenant 开启一个带租户 search_path 的事务，返回事务句柄。
// HTTP 请求链路用请求级事务（见 middleware.TenantScope），后台任务用 TenantTx。
func (m *Manager) BeginTenant(ctx context.Context, schema string) (*gorm.DB, error) {
	if err := ValidateSchema(schema); err != nil {
		return nil, err
	}
	tx := m.pool.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, tx.Error)
	}
	// search_path 带上平台 schema，租户 schema 内的表可以 JOIN 平台库的权限点字典
	if err := tx.Exec("SELECT set_config('search_path', ?, true)", schema+", "+m.platformSchema).Error; err != nil {
		_ = tx.Rollback().Error
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return tx, nil
}

// EndTenant 结束请求级事务。commit 为 false 时回滚。
func (m *Manager) EndTenant(tx *gorm.DB, commit bool) error {
	if commit {
		if err := tx.Commit().Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		return nil
	}
	if err := tx.Rollback().Error; err != nil {
		m.log.Warn("回滚租户事务失败", zap.Error(err))
	}
	return nil
}

// CreateTenantSchema 创建租户 schema，幂等。
func (m *Manager) CreateTenantSchema(ctx context.Context, schema string) error {
	if err := ValidateSchema(schema); err != nil {
		return err
	}
	// schema 名已通过正则校验，此处用 fmt 拼接而不是参数占位（DDL 不支持占位符）
	return m.pool.WithContext(ctx).Exec("CREATE SCHEMA IF NOT EXISTS " + schema).Error
}

// DropTenantSchema 删除租户 schema，仅用于测试与租户注销。
func (m *Manager) DropTenantSchema(ctx context.Context, schema string) error {
	if err := ValidateSchema(schema); err != nil {
		return err
	}
	return m.pool.WithContext(ctx).Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE").Error
}

// parseLogLevel 把配置里的日志级别映射为 gorm 的日志级别，未知值取 warn。
func parseLogLevel(level string) gormlogger.LogLevel {
	switch level {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "info":
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}
