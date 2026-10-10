package model

// TenantDictItem 租户字典项，落租户 schema（t_dict_item）。
//
// 同一字典类型下 value 唯一，唯一性由 migrate 包的租户部分唯一索引保证
// （条件 deleted_at IS NULL）。字典类型被删除时其下字典项一并软删。
type TenantDictItem struct {
	BaseModel
	DictID uint64 `gorm:"not null;index" json:"dict_id"`  // 所属租户字典类型 ID（t_dict.id）
	Label  string `gorm:"size:64;not null" json:"label"`  // 展示文案
	Value  string `gorm:"size:128;not null" json:"value"` // 实际取值，同类型下唯一（存活行）
	Sort   int    `gorm:"not null;default:0" json:"sort"`
	Status string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
	Remark string `gorm:"size:255" json:"remark"`
}

// TableName 固定表名；租户自有字典项落租户 schema。
func (TenantDictItem) TableName() string { return "t_dict_item" }
