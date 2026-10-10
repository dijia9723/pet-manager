package model

// TenantUser 租户账号（主账号 + 子账号）。
// 账号体系放在平台库以保证登录入口唯一；角色绑定则放在租户 schema 内，支持租户自定义。
type TenantUser struct {
	BaseModel
	TenantID uint64 `gorm:"not null;index" json:"tenant_id"`        // 租户隔离字段：所属租户
	Username string `gorm:"size:64;not null;index" json:"username"` // 登录名，租户内唯一（存活行）
	Password string `gorm:"size:128;not null" json:"-"`             // 口令哈希，json:"-" 保证不外泄
	Nickname string `gorm:"size:64" json:"nickname"`
	IsOwner  bool   `gorm:"not null;default:false" json:"is_owner"`               // 主账号标记，主账号不可删除
	Status   string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
}

// TableName 固定表名；账号体系落平台库以保证登录入口唯一。
func (TenantUser) TableName() string { return "sys_tenant_user" }
