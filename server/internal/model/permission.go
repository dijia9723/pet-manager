package model

// Permission 权限点字典，落平台公共库（sys_permission），由平台统一维护，租户角色只做引用。
// scope 决定它属于平台菜单还是租户菜单；type 决定它进菜单树还是只做按钮级控制。
type Permission struct {
	BaseModel
	ParentID uint64 `gorm:"not null;default:0;index" json:"parent_id"`          // 父权限点 ID，0 为根节点
	Code     string `gorm:"size:128;not null;index" json:"code"`                // 权限编码，全站唯一（存活行）
	Name     string `gorm:"size:64;not null" json:"name"`                       // 展示名
	Type     string `gorm:"size:16;not null;default:menu" json:"type"`          // 见 PermType*
	Scope    string `gorm:"size:16;not null;default:tenant;index" json:"scope"` // 归属账号体系，见 UserType*
	Path     string `gorm:"size:128" json:"path"`                               // 前端路由，按钮级权限点为空
	Icon     string `gorm:"size:64" json:"icon"`                                // 菜单图标
	Sort     int    `gorm:"not null;default:0" json:"sort"`                     // 同级升序排列
}

// TableName 固定表名；权限点为平台统一字典，落平台库。
func (Permission) TableName() string { return "sys_permission" }
