package model

// RolePermission 角色与权限点的绑定，权限点 ID 指向平台库 sys_permission。
// 纯关联表，主键为联合主键，不继承 BaseModel。
type RolePermission struct {
	RoleID       uint64 `gorm:"primaryKey" json:"role_id"`       // 角色 ID（t_role.id），联合主键
	PermissionID uint64 `gorm:"primaryKey" json:"permission_id"` // 权限点 ID（sys_permission.id），跨 schema 只存 ID
}

// TableName 固定表名；绑定关系落租户 schema，跨 schema 仅引用权限点 ID。
func (RolePermission) TableName() string { return "t_role_permission" }
