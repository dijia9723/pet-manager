package model

// PlatformUser 平台侧管理员账号，落平台公共库（sys_platform_user），只有平台运维使用。
type PlatformUser struct {
	BaseModel
	Username string `gorm:"size:64;not null;index" json:"username"` // 登录名，全局唯一（存活行）
	Password string `gorm:"size:128;not null" json:"-"`             // 口令哈希，json:"-" 保证不外泄
	Nickname string `gorm:"size:64" json:"nickname"`
	Status   string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
}

// TableName 固定表名；平台管理员属平台账号体系。
func (PlatformUser) TableName() string { return "sys_platform_user" }
