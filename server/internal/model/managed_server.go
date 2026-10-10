package model

// ManagedServer 平台纳管的服务器及其 SSH 接入凭据，落平台公共库（sys_managed_server）。
// 凭据（口令/私钥）经加密后存储，且标记 json:"-"，任何接口都不会回显。
type ManagedServer struct {
	BaseModel
	Name            string `gorm:"size:128;not null" json:"name"`                  // 服务器别名
	Host            string `gorm:"size:255;not null" json:"host"`                  // 主机地址（IP 或域名）
	Port            int    `gorm:"not null;default:22" json:"port"`                // SSH 端口
	Username        string `gorm:"size:64;not null" json:"username"`               // SSH 登录用户
	AuthType        string `gorm:"size:16;not null" json:"auth_type"`              // 认证方式：password / key
	Credential      string `gorm:"type:text;not null" json:"-"`                    // 加密后的口令或私钥，绝不外泄
	HostFingerprint string `gorm:"size:128;not null" json:"host_fingerprint"`      // 主机公钥指纹，防中间人
	Status          string `gorm:"size:16;not null;default:enabled" json:"status"` // enabled / disabled
	Remark          string `gorm:"size:255" json:"remark"`
}

// TableName 固定表名；纳管服务器属平台资源，落平台库。
func (ManagedServer) TableName() string { return "sys_managed_server" }
