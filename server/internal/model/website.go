package model

// Website 平台分配给租户的网站，落平台公共库（sys_website）。
// 归属租户与访问凭据一经分配即不可变，避免变更导致文件系统隔离目录与访问入口失配。
type Website struct {
	BaseModel
	TenantID        uint64 `gorm:"not null;index" json:"tenant_id"`                          // 归属租户，分配后不可变
	ServerID        uint64 `gorm:"not null;index" json:"server_id"`                          // 承载服务器 ID（sys_managed_server.id）
	Name            string `gorm:"size:128;not null" json:"name"`                            // 网站名称
	URL             string `gorm:"size:512" json:"url"`                                      // 站点访问地址
	CMSURL          string `gorm:"size:512" json:"cms_url"`                                  // CMS 后台访问地址
	CMS             string `gorm:"size:16;not null;default:none" json:"cms"`                 // CMS 类型，none 表示不部署
	Status          string `gorm:"size:16;not null;default:enabled" json:"status"`           // enabled / disabled
	ProvisionStatus string `gorm:"size:16;not null;default:pending" json:"provision_status"` // 隔离目录初始化状态
	SFTPKey         string `gorm:"type:text" json:"-"`                                       // SFTP 私钥，敏感、不外泄
	CMSStatus       string `gorm:"size:16;not null;default:pending" json:"cms_status"`       // CMS 部署状态
	CMSPort         int    `json:"cms_port"`                                                 // CMS 监听端口，与 server_id 组合唯一
	CMSVersion      string `gorm:"size:32" json:"cms_version"`                               // 已部署 CMS 版本
	CMSSecret       string `gorm:"type:text" json:"-"`                                       // CMS 密钥，敏感、不外泄
	Remark          string `gorm:"size:255" json:"remark"`
}

// TableName 固定表名；网站为平台分配的资源，落平台库。
func (Website) TableName() string { return "sys_website" }
