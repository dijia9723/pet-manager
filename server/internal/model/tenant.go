package model

import "time"

// Tenant 租户主表。SchemaName 由系统按 tenant_<id> 生成，不接受用户输入。
//
// 删除采用"标记 + 延迟物理回收"两段式：
//   - status=deleted + PurgeAt：待销毁宽限期（默认 7 天，sys_config 可配），
//     租户门禁按非 enabled 直接拦截，行与业务键仍占用，可一键恢复；
//   - 到期由系统定时任务 tenant_purge 执行 DROP SCHEMA 并物理删除平台侧残留行。
type Tenant struct {
	BaseModel
	Name         string     `gorm:"size:128;not null" json:"name"`             // 租户名称（店铺名）
	Code         string     `gorm:"size:64;not null;index" json:"code"`        // 租户编码，全局唯一（存活行）
	SchemaName   string     `gorm:"size:64;not null;index" json:"schema_name"` // 租户 schema 名，系统生成，不接受用户输入
	ContactName  string     `gorm:"size:64" json:"contact_name"`
	ContactPhone string     `gorm:"size:32" json:"contact_phone"`
	Address      string     `gorm:"size:255" json:"address"`
	Status       string     `gorm:"size:16;not null;default:enabled;index" json:"status"` // enabled / disabled / deleted
	ExpireAt     *time.Time `json:"expire_at"`                                            // 到期时间，过期由门禁拦截登录
	// PurgeAt 待销毁租户的物理回收时间，仅 status=deleted 时非空。
	PurgeAt *time.Time `gorm:"index" json:"purge_at"`
}

// TableName 固定表名；租户主表属平台数据，落平台库。
func (Tenant) TableName() string { return "sys_tenant" }
