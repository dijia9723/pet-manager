package model

// TenantConfig 租户参数配置，落租户 schema（t_config）。
//
// 与平台侧 sys_config 结构一致，但数据归租户自己所有：
// 每个租户可在租户端"参数配置"页维护本店业务参数，平台不感知、不互见。
// config_key 的唯一性由 migrate 包的租户部分唯一索引保证（条件 deleted_at IS NULL）。
type TenantConfig struct {
	BaseModel
	ConfigKey string `gorm:"size:128;not null" json:"config_key"` // 配置键，租户内唯一（存活行）
	Name      string `gorm:"size:64;not null" json:"name"`
	Value     string `gorm:"type:text" json:"value"`
	GroupKey  string `gorm:"size:64;not null;default:'';index" json:"group_key"`
	Type      string `gorm:"size:16;not null;default:string" json:"type"` // 见 ConfigType*
	Remark    string `gorm:"size:255" json:"remark"`
	Status    string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
}

// TableName 固定表名；租户自有配置落租户 schema，与平台 sys_config 隔离。
func (TenantConfig) TableName() string { return "t_config" }
