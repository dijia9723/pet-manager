package model

// StorageChange 记录配额分配、升级和扩容流水，落平台公共库（sys_storage_change），不涉及支付或计费订单。
type StorageChange struct {
	BaseModel
	TenantID     uint64 `gorm:"not null;index" json:"tenant_id"` // 变更所属租户
	Action       string `gorm:"size:16;not null" json:"action"`  // 操作类型（分配/升级/扩容等）
	PlanName     string `gorm:"size:64" json:"plan_name"`        // 关联套餐名称快照
	BeforeBytes  int64  `gorm:"not null" json:"before_bytes"`    // 变更前总容量（字节）
	AfterBytes   int64  `gorm:"not null" json:"after_bytes"`     // 变更后总容量（字节）
	OperatorID   uint64 `gorm:"not null" json:"operator_id"`     // 操作人 ID
	OperatorName string `gorm:"size:64" json:"operator_name"`    // 操作人名称快照
	Remark       string `gorm:"size:255" json:"remark"`
}

// TableName 固定表名；配额流水落平台库。
func (StorageChange) TableName() string { return "sys_storage_change" }
