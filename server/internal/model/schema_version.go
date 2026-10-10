package model

import "time"

// SchemaVersion 记录当前租户 schema 已执行到的迁移版本，
// 用于后续"按租户批量升级表结构"时判断增量。内部表，不做软删除。
type SchemaVersion struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Version   string    `gorm:"size:64;not null;uniqueIndex" json:"version"` // 结构版本号，唯一，重复执行不新增
	AppliedAt time.Time `gorm:"autoCreateTime" json:"applied_at"`            // 应用时间
}

// TableName 固定表名；版本表落各租户 schema，记录该租户已执行到的版本。
func (SchemaVersion) TableName() string { return "t_schema_version" }
