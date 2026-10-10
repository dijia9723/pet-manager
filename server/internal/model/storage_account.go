package model

// StorageAccount 保存租户配额快照，落平台公共库（sys_storage_account）。
// TenantID=0 是平台公共存储，不限制业务配额；每个租户至多一行（部分唯一索引保证）。
type StorageAccount struct {
	BaseModel
	TenantID   uint64 `gorm:"not null;default:0" json:"tenant_id"`   // 租户隔离字段：0 为平台公共存储
	PlanID     uint64 `gorm:"not null;default:0" json:"plan_id"`     // 套餐字典项 ID
	PlanName   string `gorm:"size:64" json:"plan_name"`              // 套餐名称快照，字典改动后仍可读
	BaseBytes  int64  `gorm:"not null;default:0" json:"base_bytes"`  // 套餐基础容量（字节）
	ExtraBytes int64  `gorm:"not null;default:0" json:"extra_bytes"` // 额外扩容容量（字节）
	UsedBytes  int64  `gorm:"not null;default:0" json:"used_bytes"`  // 已用容量（字节），由资源变更维护
}

// TableName 固定表名；配额归属平台统一管理。
func (StorageAccount) TableName() string { return "sys_storage_account" }
