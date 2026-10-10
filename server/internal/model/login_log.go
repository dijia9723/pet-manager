package model

// LoginLog 登录日志，落平台公共库（sys_login_log），同时容纳平台与租户的登录尝试。
//
// 设计取舍：
//   - tenant_id = 0 表示平台账号，非 0 为租户账号；一张表覆盖两类主体，便于统一追溯；
//   - 失败登录同样记录（安全价值主要在失败侧），Message 存失败原因；
//   - status 用 LoginStatusSuccess / LoginStatusFail 语义常量（见 consts.go），
//     与审计日志 status 用 HTTP 码 int 的口径不同，刻意不做强行统一。
//   - 复合索引 (tenant_id, created_at) / (username, created_at) 由 migrate 包补建，
//     因为 created_at 来自 BaseModel，无法在字段上声明复合索引。
type LoginLog struct {
	BaseModel
	Username   string `gorm:"size:64;not null;default:''" json:"username"`  // 登录名（尝试值，未必真实存在）
	UserType   string `gorm:"size:16;not null;default:''" json:"user_type"` // platform / tenant
	TenantID   uint64 `gorm:"not null;default:0" json:"tenant_id"`          // 租户隔离字段：0 为平台账号
	TenantCode string `gorm:"size:64" json:"tenant_code"`                   // 租户编码快照，便于按租户检索
	ClientIP   string `gorm:"size:64" json:"client_ip"`
	UserAgent  string `gorm:"size:255" json:"user_agent"`
	Status     string `gorm:"size:16;not null;default:''" json:"status"` // 见 LoginStatus*
	Message    string `gorm:"size:255" json:"message"`                   // 失败原因等说明
}

// TableName 固定表名；登录日志落平台库，不随租户切换。
func (LoginLog) TableName() string { return "sys_login_log" }
