package model

// UserRole 账号与角色的绑定，账号 ID 指向平台库 sys_tenant_user。
// 跨 schema 只存 ID、不建外键，保证租户 schema 可独立搬迁到独立库（方案 C）。
type UserRole struct {
	UserID uint64 `gorm:"primaryKey" json:"user_id"` // 账号 ID（sys_tenant_user.id），联合主键
	RoleID uint64 `gorm:"primaryKey" json:"role_id"` // 角色 ID（t_role.id）
}

// TableName 固定表名；账号角色绑定落租户 schema，跨 schema 仅引用账号 ID。
func (UserRole) TableName() string { return "t_user_role" }
