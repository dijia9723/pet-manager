package model

// TenantDict 租户字典类型，落租户 schema（t_dict）。
//
// 与平台侧 sys_dict 结构一致，但归租户自己维护，承载本店业务枚举
// （如宠物品种、服务分类）。一个类型下挂若干 t_dict_item。
// code 的唯一性由 migrate 包的租户部分唯一索引保证（条件 deleted_at IS NULL）。
type TenantDict struct {
	BaseModel
	Code   string `gorm:"size:64;not null" json:"code"` // 字典类型编码，租户内唯一（存活行）
	Name   string `gorm:"size:64;not null" json:"name"`
	Remark string `gorm:"size:255" json:"remark"`
	Status string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
}

// TableName 固定表名；租户自有字典落租户 schema，与平台 sys_dict 隔离。
func (TenantDict) TableName() string { return "t_dict" }
