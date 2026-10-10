package model

// Dict 字典类型，落平台公共库（sys_dict）。
//
// 一个字典类型下挂若干字典项（sys_dict_item），供下拉/枚举统一取值。
// code 的唯一性由 migrate 包的部分唯一索引保证（条件 deleted_at IS NULL）。
type Dict struct {
	BaseModel
	Code   string `gorm:"size:64;not null" json:"code"` // 字典类型编码，全局唯一（存活行）
	Name   string `gorm:"size:64;not null" json:"name"` // 字典类型名称
	Remark string `gorm:"size:255" json:"remark"`
	Status string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
}

// TableName 固定表名；平台字典类型落平台库。
func (Dict) TableName() string { return "sys_dict" }
