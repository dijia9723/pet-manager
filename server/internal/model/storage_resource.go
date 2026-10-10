package model

// StorageResource 存储资源元数据，落平台公共库（sys_storage_resource）；
// 物理对象统一存在平台 TOS，归属（平台或租户）由鉴权上下文决定，写入租户隔离字段。
type StorageResource struct {
	BaseModel
	TenantID     uint64 `gorm:"not null;default:0;index" json:"tenant_id"` // 租户隔离字段：0 为平台资源
	Name         string `gorm:"size:255;not null" json:"name"`             // 原始文件名
	SizeBytes    int64  `gorm:"not null" json:"size_bytes"`                // 文件大小（字节），用于配额统计
	ContentType  string `gorm:"size:128" json:"content_type"`              // MIME 类型
	Bucket       string `gorm:"size:128;not null" json:"-"`                // 对象存储桶，不对外暴露
	ObjectKey    string `gorm:"size:512;not null" json:"-"`                // 对象键，不对外暴露（下载走鉴权接口）
	Status       string `gorm:"size:16;not null;index" json:"status"`      // uploading/ready/deleting
	UploaderID   uint64 `gorm:"not null" json:"uploader_id"`               // 上传人 ID
	UploaderName string `gorm:"size:64" json:"uploader_name"`              // 上传人名称快照
}

// TableName 固定表名；资源元数据落平台库，跨租户统一管理。
func (StorageResource) TableName() string { return "sys_storage_resource" }

// Attachment 是消息冻结的附件名称与大小，不暴露对象 key 和无鉴权 URL。
type Attachment struct {
	ResourceID  uint64 `json:"resource_id"` // 指向资源元数据
	Name        string `json:"name"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}
