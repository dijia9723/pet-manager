// Package hosting manages platform SSH access and tenant-owned, chrooted websites.
package hosting

import (
	"context"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
)

// Service 管理平台 SSH 服务器与租户网站，负责凭证加密、受限访问与部署编排。
type Service struct {
	manager         *db.Manager
	cipher          cipher.AEAD // 凭证/密钥的 AES-GCM 加解密器，未配置密钥时为 nil
	terminalOrigins []string    // 终端 WebSocket 允许的 Origin 白名单
}

// NewService 构造托管服务；未配置加密密钥时凭证相关操作会返回不可用错误。
func NewService(m *db.Manager, cfg config.HostingConfig, allowedOrigins ...string) (*Service, error) {
	s := &Service{manager: m, terminalOrigins: append([]string(nil), allowedOrigins...)}
	if cfg.EncryptionKey != "" {
		var err error
		s.cipher, err = newCipher(cfg.EncryptionKey)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

// ListQuery 是服务器/网站列表的通用查询参数，TenantID 仅平台侧生效。
type ListQuery struct {
	pagination.Query
	Keyword  string `form:"keyword"`
	TenantID uint64 `form:"tenant_id"`
}

// ServerInput 是新增/编辑受管服务器入参；Credential 为空表示沿用原凭证。
type ServerInput struct {
	Name            string `json:"name" binding:"required,max=128"`
	Host            string `json:"host" binding:"required,max=255"`
	Port            int    `json:"port" binding:"required,min=1,max=65535"`
	Username        string `json:"username" binding:"required,max=64"`
	AuthType        string `json:"auth_type" binding:"required,oneof=key password"`
	Credential      string `json:"credential" binding:"max=65536"`
	HostFingerprint string `json:"host_fingerprint" binding:"required,max=1024"`
	Status          string `json:"status" binding:"required,oneof=enabled disabled"`
	Remark          string `json:"remark" binding:"max=255"`
}

// WebsiteInput 是分配/编辑网站入参；归属租户与服务器一经创建不可更改。
type WebsiteInput struct {
	TenantID uint64 `json:"tenant_id" binding:"required"`
	ServerID uint64 `json:"server_id" binding:"required"`
	Name     string `json:"name" binding:"required,max=128"`
	URL      string `json:"url" binding:"max=512"`
	CMSURL   string `json:"cms_url" binding:"max=512"`
	CMS      string `json:"cms" binding:"required,oneof=none directus"`
	Status   string `json:"status" binding:"required,oneof=enabled disabled"`
	Remark   string `json:"remark" binding:"max=255"`
}

// ListServers 分页查询受管服务器，支持按名称/主机模糊筛选。
func (s *Service) ListServers(ctx context.Context, q ListQuery) (*response.Page[model.ManagedServer], error) {
	tx := s.manager.Platform().WithContext(ctx).Model(&model.ManagedServer{})
	if q.Keyword != "" {
		tx = tx.Where("name ILIKE ? OR host ILIKE ?", "%"+q.Keyword+"%", "%"+q.Keyword+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	var rows []model.ManagedServer
	err := tx.Order("id desc").Limit(q.Limit()).Offset(q.Offset()).Find(&rows).Error
	return response.NewPage(rows, total), dbError(err)
}

// dbError 归一化数据库错误：不存在转 404，业务错误原样，其余包装为库错误。
func dbError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.New(errcode.ErrNotFound)
	}
	var business *errcode.Error
	if errors.As(err, &business) {
		return business
	}
	return errcode.Wrap(errcode.ErrDatabase, err)
}

// server 按 ID 读取受管服务器。
func (s *Service) server(ctx context.Context, id uint64) (*model.ManagedServer, error) {
	var row model.ManagedServer
	err := s.manager.Platform().WithContext(ctx).First(&row, id).Error
	return &row, dbError(err)
}

// SaveServer 新增或编辑受管服务器：校验主机/指纹/凭证，凭证加密存储；
// 编辑时以行锁保护，已有网站的服务器不允许更换主机、端口或指纹。
func (s *Service) SaveServer(ctx context.Context, id uint64, in ServerInput) (*model.ManagedServer, error) {
	in.Host = strings.TrimSpace(in.Host)
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || strings.ContainsAny(in.Host, "/\\@ \t\r\n") || in.Host == "" || (strings.Contains(in.Host, ":") && net.ParseIP(in.Host) == nil) {
		return nil, errcode.New(errcode.ErrInvalidParam)
	}
	fingerprint, fingerprintErr := normalizeFingerprint(in.HostFingerprint)
	if fingerprintErr != nil {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "主机指纹无效，请填写 SHA256 指纹值或粘贴 ssh-keygen -lf 的单行输出")
	}
	in.HostFingerprint = fingerprint
	if in.Credential != "" && in.AuthType == "key" {
		if _, err := ssh.ParsePrivateKey([]byte(in.Credential)); err != nil {
			return nil, errcode.Newf(errcode.ErrInvalidParam, "私钥格式不合法")
		}
	}
	row := &model.ManagedServer{}
	if id > 0 {
		var err error
		row, err = s.server(ctx, id)
		if err != nil {
			return nil, err
		}
		if in.Credential == "" && in.AuthType != row.AuthType {
			return nil, errcode.Newf(errcode.ErrInvalidParam, "更换认证方式时必须提供凭证")
		}
	}
	if id == 0 && in.Credential == "" {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "SSH 凭证不能为空")
	}
	if in.Credential != "" {
		encrypted, err := s.encrypt(in.Credential)
		if err != nil {
			return nil, err
		}
		row.Credential = encrypted
	}
	row.Name = in.Name
	row.Host = in.Host
	row.Port = in.Port
	row.Username = in.Username
	row.AuthType = in.AuthType
	row.HostFingerprint = in.HostFingerprint
	row.Status = in.Status
	row.Remark = in.Remark
	// Lock the server row across update/delete/allocation so an allocation cannot race deletion.
	err := s.manager.Platform().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if id > 0 {
			var locked model.ManagedServer
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, id).Error; err != nil {
				return err
			}
			if in.Credential == "" {
				if in.AuthType != locked.AuthType {
					return errcode.Newf(errcode.ErrInvalidParam, "更换认证方式时必须提供凭证")
				}
				row.Credential = locked.Credential
			}
			var count int64
			if err := tx.Model(&model.Website{}).Where("server_id=?", id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 && (locked.Host != row.Host || locked.Port != row.Port || locked.HostFingerprint != row.HostFingerprint) {
				return errcode.Newf(errcode.ErrConflict, "已有网站的服务器不能更换主机、端口或指纹，请新建服务器")
			}
		}
		return tx.Save(row).Error
	})
	return row, dbError(err)
}

// DeleteServer 删除未关联任何网站的服务器，避免留下悬空引用。
func (s *Service) DeleteServer(ctx context.Context, id uint64) error {
	return dbError(s.manager.Platform().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var server model.ManagedServer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&server, id).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&model.Website{}).Where("server_id=?", id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errcode.Newf(errcode.ErrConflict, "请先处理此服务器关联的网站")
		}
		return tx.Delete(&server).Error
	}))
}

// Execute 用平台账号登录服务器执行一条命令，带 30s 超时，返回输出与退出码。
func (s *Service) Execute(ctx context.Context, id uint64, command string) (*CommandResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	server, err := s.server(ctx, id)
	if err != nil {
		return nil, err
	}
	if server.Status != "enabled" {
		return nil, errcode.Newf(errcode.ErrConflict, "服务器已停用")
	}
	secret, err := s.decrypt(server.Credential)
	if err != nil {
		return nil, err
	}
	client, closeConn, err := dial(ctx, *server, server.Username, server.AuthType, secret)
	if err != nil {
		return nil, err
	}
	defer closeConn()
	return run(client, command, nil)
}

// ListWebsites 分页查询网站：先回收中断的部署状态，再按身份作用域过滤；
// 租户视角隐藏服务器 ID 与 CMS 端口等平台信息。
func (s *Service) ListWebsites(ctx context.Context, q ListQuery) (*response.Page[model.Website], error) {
	if err := s.RecoverInterrupted(ctx); err != nil {
		return nil, err
	}
	tx, err := scopeWebsites(ctx, s.manager.Platform().WithContext(ctx).Model(&model.Website{}))
	if err != nil {
		return nil, err
	}
	if ctxkey.UserType(ctx) == ctxkey.UserTypePlatform && q.TenantID > 0 {
		tx = tx.Where("tenant_id=?", q.TenantID)
	}
	if q.Keyword != "" {
		tx = tx.Where("name ILIKE ? OR url ILIKE ?", "%"+q.Keyword+"%", "%"+q.Keyword+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, dbError(err)
	}
	var rows []model.Website
	err = tx.Order("id desc").Limit(q.Limit()).Offset(q.Offset()).Find(&rows).Error
	if ctxkey.UserType(ctx) == ctxkey.UserTypeTenant {
		for i := range rows {
			rows[i].ServerID = 0
			rows[i].CMSPort = 0
		}
	}
	return response.NewPage(rows, total), dbError(err)
}

// validURL 校验网站地址：空值允许；否则必须为无账号、无查询/锚点的 HTTP(S) 地址。
func validURL(value string) bool {
	if value == "" {
		return true
	}
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && !strings.ContainsAny(value, "\r\n")
}

// SaveWebsite 分配或编辑网站：校验租户/服务器有效性，新建时为站点生成 SFTP 密钥；
// 归属与服务器不可更改，部署中或已就绪的 CMS 类型受保护。
func (s *Service) SaveWebsite(ctx context.Context, id uint64, in WebsiteInput) (*model.Website, error) {
	if strings.TrimSpace(in.Name) == "" || !validURL(in.URL) || !validURL(in.CMSURL) {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "名称或网站/CMS 地址不合法，仅支持 HTTP(S)")
	}
	row := &model.Website{ProvisionStatus: "pending", CMSStatus: "pending"}
	err := s.manager.Platform().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant model.Tenant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&tenant, in.TenantID).Error; err != nil {
			return err
		}
		if tenant.Status == model.StatusDeleted {
			return errcode.Newf(errcode.ErrConflict, "待销毁租户不能分配网站")
		}
		var server model.ManagedServer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&server, in.ServerID).Error; err != nil {
			return err
		}
		if id > 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(row, id).Error; err != nil {
				return err
			}
			if row.TenantID != in.TenantID || row.ServerID != in.ServerID {
				return errcode.Newf(errcode.ErrConflict, "网站归属和服务器不可修改")
			}
			if row.ProvisionStatus == "provisioning" || row.CMSStatus == "deploying" {
				return errcode.Newf(errcode.ErrConflict, "网站正在部署，请稍后编辑")
			}
			if row.CMSStatus == "ready" && row.CMS != in.CMS {
				return errcode.Newf(errcode.ErrConflict, "已部署的 CMS 类型不可修改")
			}
		} else {
			_, key, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				return err
			}
			block, err := ssh.MarshalPrivateKey(key, "pet-manager website SFTP")
			if err != nil {
				return err
			}
			row.SFTPKey, err = s.encrypt(string(pem.EncodeToMemory(block)))
			if err != nil {
				return err
			}
		}
		row.TenantID = in.TenantID
		row.ServerID = in.ServerID
		row.Name = strings.TrimSpace(in.Name)
		row.URL = strings.TrimRight(in.URL, "/")
		row.CMSURL = strings.TrimRight(in.CMSURL, "/")
		row.CMS = in.CMS
		row.Status = in.Status
		row.Remark = in.Remark
		return tx.Save(row).Error
	})
	return row, dbError(err)
}

// website scopes every lookup by the authenticated tenant; no caller-supplied ownership is trusted.
func (s *Service) website(ctx context.Context, id uint64) (*model.Website, error) {
	tx, err := scopeWebsites(ctx, s.manager.Platform().WithContext(ctx))
	if err != nil {
		return nil, err
	}
	var row model.Website
	err = tx.First(&row, id).Error
	return &row, dbError(err)
}

// scopeWebsites 按登录身份追加网站可见性过滤：租户只能看本租户，平台放行，其余拒绝。
func scopeWebsites(ctx context.Context, tx *gorm.DB) (*gorm.DB, error) {
	switch ctxkey.UserType(ctx) {
	case ctxkey.UserTypeTenant:
		if ctxkey.TenantID(ctx) == 0 {
			return nil, errcode.New(errcode.ErrForbiddenAction)
		}
		return tx.Where("tenant_id=?", ctxkey.TenantID(ctx)), nil
	case ctxkey.UserTypePlatform:
		return tx, nil
	default:
		return nil, errcode.New(errcode.ErrForbiddenAction)
	}
}
