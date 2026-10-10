package model

// DictItem 字典项，落平台公共库（sys_dict_item）。
//
// 同一字典类型下 value 唯一，唯一性由 migrate 包的部分唯一索引保证
// （条件 deleted_at IS NULL）。字典类型被删除时其下字典项一并软删。
type DictItem struct {
	BaseModel
	DictID       uint64 `gorm:"not null;index" json:"dict_id"`                        // 所属字典类型 ID（sys_dict.id）
	Label        string `gorm:"size:64;not null" json:"label"`                        // 展示文案
	Value        string `gorm:"size:128;not null" json:"value"`                       // 实际取值，同类型下唯一（存活行）
	Sort         int    `gorm:"not null;default:0" json:"sort"`                       // 升序排列
	Status       string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
	Remark       string `gorm:"size:255" json:"remark"`
	StorageBytes int64  `gorm:"not null;default:0" json:"storage_bytes"` // 附带容量，仅存储套餐字典使用
}

// TableName 固定表名；平台字典项落平台库。
func (DictItem) TableName() string { return "sys_dict_item" }
