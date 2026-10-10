package model

import "time"

// Notification 是平台或租户发布的站内通知，落平台公共库（sys_notification）；
// 发布主体（平台/租户）从鉴权上下文写入，二者共用一张表。
type Notification struct {
	BaseModel
	Title          string       `gorm:"size:128;not null" json:"title"`                                                                // 标题
	Content        string       `gorm:"type:text;not null" json:"content"`                                                             // 正文
	Category       string       `gorm:"size:16;not null" json:"category"`                                                              // 分类
	Priority       string       `gorm:"size:16;not null" json:"priority"`                                                              // 优先级
	RequireProcess bool         `gorm:"not null;default:false" json:"require_process"`                                                 // 是否要求接收人回执处理
	TargetType     string       `gorm:"size:32;not null" json:"target_type"`                                                           // 接收范围类型，配合 TargetIDs 展开
	TargetIDs      []uint64     `gorm:"serializer:json;type:jsonb" json:"target_ids"`                                                  // 目标 ID 列表，JSON 序列化
	Status         string       `gorm:"size:16;not null;index" json:"status"`                                                          // 草稿 / 已发布 / 已撤回
	SenderID       uint64       `gorm:"not null" json:"sender_id"`                                                                     // 发送人 ID
	SenderName     string       `gorm:"size:64" json:"sender_name"`                                                                    // 发送人名称快照
	SenderType     string       `gorm:"size:16;not null;default:platform;index:idx_notification_sender,priority:1" json:"sender_type"` // platform / tenant，复合索引首列
	SenderTenantID uint64       `gorm:"not null;default:0;index:idx_notification_sender,priority:2" json:"sender_tenant_id"`           // 发送方租户，平台发布时为 0
	PublishedAt    *time.Time   `json:"published_at"`                                                                                  // 发布时间，草稿为空
	Attachments    []Attachment `gorm:"serializer:json;type:jsonb" json:"attachments"`                                                 // 冻结的附件快照，JSON 序列化
}

// TableName 固定表名；通知主体落平台库，租户通知靠 sender_tenant_id 区分。
func (Notification) TableName() string { return "sys_notification" }
