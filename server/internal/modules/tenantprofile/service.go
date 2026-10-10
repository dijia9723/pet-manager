// Package tenantprofile 承载租户端"自己的信息"。
//
// 只暴露租户可自助维护的基础资料（名称/联系人/电话/地址）；
// 状态与到期时间属于平台运营口径，租户端改不了。
package tenantprofile

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/router"
)

// Item 与后台契约一致。
type Item struct {
	ID           uint64     `json:"id"`            // 租户 ID
	Name         string     `json:"name"`          // 店铺名称
	Code         string     `json:"code"`          // 租户编码
	ContactName  string     `json:"contact_name"`  // 联系人
	ContactPhone string     `json:"contact_phone"` // 联系电话
	Address      string     `json:"address"`       // 地址
	ExpireAt     *time.Time `json:"expire_at"`     // 到期时间（只读）
	Status       string     `json:"status"`        // 状态（只读）
}

// UpdateInput 只接受租户可自助修改的字段。
// 状态与到期时间属于平台运营口径，租户端改不了（这也是为什么 payload 里带了它们却不生效）。
type UpdateInput struct {
	Name         string `json:"name" binding:"required,max=128"`
	ContactName  string `json:"contact_name" binding:"max=64"`
	ContactPhone string `json:"contact_phone" binding:"max=32"`
	Address      string `json:"address" binding:"max=255"`
}

// Service 承载租户端自助维护店铺资料，同时作为 router.Module 注册路由。
type Service struct {
	manager  *db.Manager    // 数据库管理器
	sessions *session.Store // 会话与租户快照缓存
	log      *zap.Logger    // 日志
}

// NewService 组装店铺资料服务。
func NewService(manager *db.Manager, sessions *session.Store, log *zap.Logger) *Service {
	return &Service{manager: manager, sessions: sessions, log: log}
}

// Name 实现 router.Module。
func (s *Service) Name() string { return "tenant.profile" }

// Register 挂载租户端店铺资料接口。
func (s *Service) Register(r *router.Router) {
	g := r.Tenant("/info")
	g.GET("", r.Require("tenant.profile.view"), s.handleGet)
	g.PUT("", r.Require("tenant.profile.update"), s.handleUpdate)
}

// Get 返回当前租户店铺资料。
func (s *Service) Get(ctx context.Context) (*Item, error) {
	t, err := s.load(ctx, ctxkey.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	item := toItem(t)
	return &item, nil
}

// Update 更新租户可自助维护的字段，并失效租户快照缓存。
func (s *Service) Update(ctx context.Context, in UpdateInput) (*Item, error) {
	tenantID := ctxkey.TenantID(ctx)
	t, err := s.load(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	t.Name = in.Name
	t.ContactName = in.ContactName
	t.ContactPhone = in.ContactPhone
	t.Address = in.Address
	if err := s.manager.Platform().WithContext(ctx).Save(t).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	// 店铺名称随租户快照缓存，改了要立刻失效
	if err := s.sessions.InvalidateTenant(ctx, tenantID); err != nil {
		s.log.Warn("失效租户缓存失败", zap.Uint64("tenant_id", tenantID), zap.Error(err))
	}
	item := toItem(t)
	return &item, nil
}

// load 从平台库读取当前租户记录。
func (s *Service) load(ctx context.Context, tenantID uint64) (*model.Tenant, error) {
	if tenantID == 0 {
		return nil, errcode.New(errcode.ErrTenantNotFound)
	}
	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).First(&t, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrTenantNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return &t, nil
}

// toItem 把租户模型映射为返回结构。
func toItem(t *model.Tenant) Item {
	return Item{
		ID:           t.ID,
		Name:         t.Name,
		Code:         t.Code,
		ContactName:  t.ContactName,
		ContactPhone: t.ContactPhone,
		Address:      t.Address,
		ExpireAt:     t.ExpireAt,
		Status:       t.Status,
	}
}

// handleGet 查看本租户店铺资料
//
//	@Summary		店铺资料
//	@Description	返回当前租户可自助维护的基础资料（只读字段 status/expire_at 由平台运营管理）
//	@Tags			租户-店铺资料
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Body	"data 为 Item"
//	@Router			/tenant/info [get]
func (s *Service) handleGet(c *gin.Context) {
	item, err := s.Get(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleUpdate 更新本租户店铺资料
//
//	@Summary		更新店铺资料
//	@Description	仅名称/联系人/电话/地址可改；更新后失效租户快照缓存
//	@Tags			租户-店铺资料
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		UpdateInput		true	"店铺资料"
//	@Success		200		{object}	response.Body	"data 为 Item"
//	@Router			/tenant/info [put]
func (s *Service) handleUpdate(c *gin.Context) {
	var in UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.Update(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}
