package model

import "time"

// NotificationReceipt 保存发布时的接收人快照及账号级已读/处理回执，落平台公共库（sys_notification_receipt）。
// 发布瞬间按接收范围展开成每人一行，避免后续范围变化影响历史回执归属。
type NotificationReceipt struct {
	BaseModel
	NotificationID uint64     `gorm:"not null;index" json:"notification_id"`                                       // 关联通知 ID
	UserType       string     `gorm:"size:16;not null;index:idx_notification_inbox,priority:1" json:"user_type"`   // platform / tenant，收件箱索引首列
	TenantID       uint64     `gorm:"not null;default:0;index:idx_notification_inbox,priority:2" json:"tenant_id"` // 接收人所属租户，平台账号为 0
	UserID         uint64     `gorm:"not null;index:idx_notification_inbox,priority:3" json:"user_id"`             // 接收人 ID
	Username       string     `gorm:"size:64" json:"username"`                                                     // 接收人登录名快照
	TenantName     string     `gorm:"size:128" json:"tenant_name"`                                                 // 接收人租户名称快照
	ReadAt         *time.Time `json:"read_at"`                                                                     // 已读时间，未读为空
	ProcessedAt    *time.Time `json:"processed_at"`                                                                // 处理完成时间，仅需回执的通知使用
	Feedback       string     `gorm:"type:text" json:"feedback"`                                                   // 处理反馈内容
}

// TableName 固定表名；回执落平台库，便于跨租户统一查询。
func (NotificationReceipt) TableName() string { return "sys_notification_receipt" }
