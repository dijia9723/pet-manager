package model

// Config 平台参数配置，落平台公共库（sys_config）。
//
// 用于承载"不改代码即可调整"的系统级参数（如站点名称、安全策略开关）。
// 注意列名用 group_key 而非 group：group 是 PostgreSQL 保留字，
// 裸 SQL 或 Where 条件里极易踩坑，从一开始就避开。
// config_key 的唯一性由 migrate 包的部分唯一索引保证（条件 deleted_at IS NULL）。
type Config struct {
	BaseModel
	ConfigKey string `gorm:"size:128;not null" json:"config_key"`                  // 配置键，代码读取时使用的唯一标识
	Name      string `gorm:"size:64;not null" json:"name"`                         // 展示名
	Value     string `gorm:"type:text" json:"value"`                               // 配置值，统一以字符串存储，按 Type 解析
	GroupKey  string `gorm:"size:64;not null;default:'';index" json:"group_key"`   // 分组键，用于参数页归类
	Type      string `gorm:"size:16;not null;default:string" json:"type"`          // 取值类型，见 consts.go 的 ConfigType*
	Remark    string `gorm:"size:255" json:"remark"`                               // 说明，展示给管理员
	Status    string `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled
}

// TableName 固定表名；平台参数配置落平台库。
func (Config) TableName() string { return "sys_config" }
