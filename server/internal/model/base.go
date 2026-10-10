// Package model 定义数据模型。
//
// 表命名约定：
//   - sys_*：平台公共库（public schema），永不随租户切换；
//   - t_*  ：租户业务表，位于 tenant_<id> schema，通过 TenantTx / TenantScope 访问。
//
// 组织约定：一个表一个文件，文件名即表名去掉前缀（如 TenantUser -> tenant_user.go）。
// 所有模型统一嵌入 BaseModel（ID / CreatedAt / UpdatedAt / DeletedAt）；
// 纯关联表（RolePermission / UserRole）与内部版本表（SchemaVersion）例外，它们不参与软删除。
//
// 新增表时除了本文件，还要同步：
//   - 在对应的聚合列表（models.go）中登记，否则 AutoMigrate 不会建表；
//   - 若有自然键唯一性要求，到 migrate 包补一条 "WHERE deleted_at IS NULL" 的部分唯一索引。
package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 是所有数据表的公共字段：
// ID 主键、CreatedAt 创建时间、UpdatedAt 更新时间、DeletedAt 软删除标记。
//
// 平台库（sys_*）与租户库（t_*）的模型统一嵌入它，保证审计字段口径一致。
// 注意：使用软删除后，自然键的唯一性由"部分唯一索引"保证
// （见 migrate 包，索引条件 deleted_at IS NULL），
// 这样已删除的历史行不会永久占用用户名 / 编码这类业务键。
type BaseModel struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// SetID 供测试与种子数据使用。
func (b *BaseModel) SetID(id uint64) { b.ID = id }
