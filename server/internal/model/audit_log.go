package model

// AuditLog 审计日志，落平台公共库（sys_audit_log），同时容纳平台与租户的写操作事件。
//
// 设计取舍：
//   - tenant_id = 0 表示平台事件，非 0 为租户事件，一张表覆盖两类主体，便于统一追溯；
//   - 刻意不记录请求体/响应体：重置密码等接口的明文口令绝不能进审计表，
//     追溯依赖 route + status + 操作人 + request_id 回查访问日志；
//   - 复合索引 (tenant_id, created_at) / (user_id, created_at) 由 migrate 包补建，
//     因为 created_at 来自 BaseModel，无法在字段上声明复合索引。
type AuditLog struct {
	BaseModel
	TenantID  uint64 `gorm:"not null;default:0" json:"tenant_id"`          // 租户隔离字段：0 为平台事件，非 0 为租户事件
	UserType  string `gorm:"size:16;not null;default:''" json:"user_type"` // platform / tenant，标识操作人所属账号体系
	UserID    uint64 `gorm:"not null;default:0" json:"user_id"`            // 操作人主键，配合 UserType 定位账号
	Username  string `gorm:"size:64" json:"username"`                      // 操作人登录名快照，账号删除后仍可追溯
	Method    string `gorm:"size:16;not null" json:"method"`               // HTTP 方法
	Path      string `gorm:"size:255;not null" json:"path"`                // 实际请求路径（含动态段）
	Route     string `gorm:"size:128" json:"route"`                        // 路由模板，便于按接口维度聚合
	Status    int    `gorm:"not null;default:0" json:"status"`             // HTTP 状态码，非业务语义值
	ClientIP  string `gorm:"size:64" json:"client_ip"`
	UserAgent string `gorm:"size:255" json:"user_agent"`
	LatencyMs int64  `gorm:"not null;default:0" json:"latency_ms"` // 请求处理耗时（毫秒）
	RequestID string `gorm:"size:64" json:"request_id"`            // 请求链路 ID，用于回查访问日志
}

// TableName 固定表名；审计日志统一落平台库，不随租户切换。
func (AuditLog) TableName() string { return "sys_audit_log" }
