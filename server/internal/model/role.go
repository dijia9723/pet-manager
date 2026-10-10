package model

// Role 租户自定义角色。IsSystem 为 true 的角色由系统预置（主账号 / 店员），
// 不允许删除，也不允许改动 Code。
type Role struct {
	BaseModel
	Name     string `gorm:"size:64;not null" json:"name"`            // 角色名称
	Code     string `gorm:"size:64;not null;index" json:"code"`      // 角色编码，租户内唯一（存活行）
	IsSystem bool   `gorm:"not null;default:false" json:"is_system"` // 系统预置角色标记
	Remark   string `gorm:"size:255" json:"remark"`
}

// TableName 固定表名；角色是租户自有数据，落租户 schema。
func (Role) TableName() string { return "t_role" }

// 预置角色编码。owner 的判定逻辑散落在权限解析与账号保护里，因此集中在这里定义。
const (
	RoleCodeOwner = "owner" // 租户主账号（老板），拥有全部权限
	RoleCodeStaff = "staff" // 店员，默认仅基础业务权限
	RoleNameOwner = "主账号"
	RoleNameStaff = "店员"
)
