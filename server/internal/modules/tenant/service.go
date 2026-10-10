// Package tenant 承载平台侧的租户管理：列表/详情/新建/修改/启停，
// 以及"开通租户"这一需要跨库协同的编排动作。
//
// 开通租户是本模块唯一有状态的流程，各步骤都可能中途失败，因此顺序被刻意设计成
// "可补偿"的形态：
//
//  1. 平台库插入租户记录（拿到自增 ID，schema 名由 ID 推导）
//  2. CREATE SCHEMA + AutoMigrate 租户表
//  3. 在租户 schema 内预置角色（主账号 / 店员）
//  4. 创建租户主账号并绑定主账号角色
//
// 第 2 步之后任一步失败，都会执行补偿：删掉 schema、删掉租户记录，
// 避免留下"有租户没有表"或"有表没有账号"的半成品。
package tenant

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"pet-manager/server/internal/infra/cache"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/migrate"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/modules/rbac"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/pkg/security"
)

const (
	// purgeGraceDaysKey 是租户删除宽限天数的参数键（sys_config，group_key=tenant）。
	// 删除后只标记不物理回收，到期由系统任务 tenant_purge 销毁 schema。
	purgeGraceDaysKey = "tenant.purge_after_days"
	// defaultPurgeGraceDays 是参数缺失/非法时的兜底宽限期（7 天）。
	defaultPurgeGraceDays = 7
)

// —— DTO ——

// Item 是租户列表/详情返回结构，字段与后台契约一一对应。
type Item struct {
	ID           uint64     `json:"id"`            // 租户 ID
	Name         string     `json:"name"`          // 租户名称
	Code         string     `json:"code"`          // 租户编码（业务唯一键）
	ContactName  string     `json:"contact_name"`  // 联系人
	ContactPhone string     `json:"contact_phone"` // 联系电话
	Status       string     `json:"status"`        // enabled / disabled / deleted
	ExpireAt     *time.Time `json:"expire_at"`     // 到期时间
	PurgeAt      *time.Time `json:"purge_at"`      // 待销毁时间（宽限期到期点）
	CreatedAt    time.Time  `json:"created_at"`    // 创建时间
}

// ListQuery 是租户列表查询条件。
type ListQuery struct {
	pagination.Query
	Keyword string `form:"keyword"` // 名称/编码/联系人/电话关键字
	Status  string `form:"status"`  // 状态筛选
}

// OwnerInput 是开通租户时同步创建的主账号。
type OwnerInput struct {
	Username string `json:"username" binding:"required,min=3,max=64"` // 主账号登录名
	Password string `json:"password" binding:"required,min=6,max=64"` // 主账号口令
	Nickname string `json:"nickname" binding:"max=64"`                // 主账号昵称
}

// CreateInput 是开通租户入参。
type CreateInput struct {
	Name         string     `json:"name" binding:"required,max=128"` // 租户名称
	Code         string     `json:"code" binding:"required,max=64"`  // 租户编码
	ContactName  string     `json:"contact_name" binding:"max=64"`   // 联系人
	ContactPhone string     `json:"contact_phone" binding:"max=32"`  // 联系电话
	ExpireAt     *time.Time `json:"expire_at"`                       // 到期时间
	Owner        OwnerInput `json:"owner" binding:"required"`        // 随租户创建的主账号
}

// UpdateInput 是编辑租户基础资料入参。
type UpdateInput struct {
	Name         string `json:"name" binding:"required,max=128"` // 租户名称
	Code         string `json:"code" binding:"required,max=64"`  // 租户编码
	ContactName  string `json:"contact_name" binding:"max=64"`   // 联系人
	ContactPhone string `json:"contact_phone" binding:"max=32"`  // 联系电话
}

// —— Service ——

// Service 同时承担三件事：租户业务的实现、router.Module 的路由注册、
// 以及 middleware.TenantResolver（请求链路做租户门禁时取租户快照）。
type Service struct {
	manager  *db.Manager    // 数据库管理器
	sessions *session.Store // 会话与租户/权限缓存
	cache    *cache.Client  // Redis 客户端（开通锁等）
	rbac     *rbac.Service  // 预置角色依赖
	log      *zap.Logger    // 日志
}

// NewService 组装租户管理服务。
func NewService(manager *db.Manager, sessions *session.Store, c *cache.Client, rbacSvc *rbac.Service, log *zap.Logger) *Service {
	return &Service{manager: manager, sessions: sessions, cache: c, rbac: rbacSvc, log: log}
}

// TenantByID 实现 middleware.TenantResolver，带 Redis 快照缓存。
func (s *Service) TenantByID(ctx context.Context, id uint64) (*session.TenantInfo, error) {
	if info, ok := s.sessions.CachedTenant(ctx, id); ok {
		return info, nil
	}

	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrTenantNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	info := &session.TenantInfo{
		ID:       t.ID,
		Code:     t.Code,
		Schema:   t.SchemaName,
		Name:     t.Name,
		Status:   t.Status,
		ExpireAt: t.ExpireAt,
	}
	if err := s.sessions.CacheTenant(ctx, *info); err != nil {
		s.log.Warn("缓存租户快照失败", zap.Uint64("tenant_id", id), zap.Error(err))
	}
	return info, nil
}

// —— 平台端租户管理 ——

// List 平台端分页查询租户，支持关键字与状态筛选。
func (s *Service) List(ctx context.Context, q ListQuery) (*response.Page[Item], error) {
	tx := s.manager.Platform().WithContext(ctx).Model(&model.Tenant{})
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("name ILIKE ? OR code ILIKE ? OR contact_name ILIKE ? OR contact_phone ILIKE ?", like, like, like, like)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	var rows []model.Tenant
	if err := tx.Order("id desc").Offset(q.Offset()).Limit(q.Limit()).Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return response.NewPage(toItems(rows), total), nil
}

// Detail 返回单个租户详情。
func (s *Service) Detail(ctx context.Context, id uint64) (*Item, error) {
	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrTenantNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	items := toItems([]model.Tenant{t})
	return &items[0], nil
}

// Create 开通租户。整体流程见包注释。
func (s *Service) Create(ctx context.Context, in CreateInput) (*Item, error) {
	code := strings.TrimSpace(in.Code)

	// 同一编码的开通动作串行化，避免并发创建出两个同编码租户
	lockKey := s.cache.Key("lock", "tenant:create", code)
	ok, err := s.cache.AcquireLock(ctx, lockKey, 30*time.Second)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if !ok {
		return nil, errcode.New(errcode.ErrConflict)
	}
	defer func() {
		if err := s.cache.ReleaseLock(ctx, lockKey); err != nil {
			s.log.Warn("释放租户开通锁失败", zap.Error(err))
		}
	}()

	if err := s.ensureCodeAvailable(ctx, code, 0); err != nil {
		return nil, err
	}

	// 1. 平台库先落租户记录，拿到 ID 才能推导 schema 名
	t := model.Tenant{
		Name:         in.Name,
		Code:         code,
		ContactName:  in.ContactName,
		ContactPhone: in.ContactPhone,
		ExpireAt:     in.ExpireAt,
		Status:       model.StatusEnabled,
	}
	if err := s.manager.Platform().WithContext(ctx).Create(&t).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	schema := db.SchemaForTenant(t.ID)
	t.SchemaName = schema
	if err := s.manager.Platform().WithContext(ctx).Model(&model.Tenant{}).
		Where("id = ?", t.ID).Update("schema_name", schema).Error; err != nil {
		s.compensate(ctx, t.ID, "")
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	// 2~4. 建 schema、建表、预置角色、建主账号
	if err := s.provision(ctx, schema, t, in.Owner); err != nil {
		s.compensate(ctx, t.ID, schema)
		return nil, err
	}

	s.log.Info("租户已开通",
		zap.Uint64("tenant_id", t.ID),
		zap.String("code", t.Code),
		zap.String("schema", schema),
		zap.String("owner", in.Owner.Username),
	)

	items := toItems([]model.Tenant{t})
	return &items[0], nil
}

// provision 在租户 schema 内完成建表、预置角色、创建主账号。
func (s *Service) provision(ctx context.Context, schema string, t model.Tenant, owner OwnerInput) error {
	if err := migrate.TenantSchema(ctx, s.manager, schema); err != nil {
		return err
	}

	hashed, err := security.HashPassword(owner.Password)
	if err != nil {
		return errcode.Wrap(errcode.ErrInternal, err)
	}

	return s.manager.TenantTx(ctx, schema, func(tx *gorm.DB) error {
		roles, err := s.rbac.SeedTenantRoles(ctx, tx)
		if err != nil {
			return err
		}

		// 账号表在平台库（public），search_path 里带上 public 所以同一事务内可直接写，
		// 这样"账号 + 角色绑定"天然要么全成、要么全不成。
		account := model.TenantUser{
			TenantID: t.ID,
			Username: owner.Username,
			Password: hashed,
			Nickname: owner.Nickname,
			IsOwner:  true,
			Status:   model.StatusEnabled,
		}
		if err := tx.Create(&account).Error; err != nil {
			if isUniqueViolation(err) {
				return errcode.New(errcode.ErrUsernameTaken)
			}
			return errcode.Wrap(errcode.ErrDatabase, fmt.Errorf("创建租户主账号失败: %w", err))
		}

		ownerRoleID, ok := roles[model.RoleCodeOwner]
		if !ok {
			return errcode.New(errcode.ErrInternal)
		}
		if err := tx.Create(&model.UserRole{UserID: account.ID, RoleID: ownerRoleID}).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		return nil
	})
}

// compensate 回滚开通动作：删 schema（含表）与租户记录。
// 补偿失败只记日志，不掩盖原始错误。
func (s *Service) compensate(ctx context.Context, tenantID uint64, schema string) {
	if schema != "" {
		if err := s.manager.DropTenantSchema(ctx, schema); err != nil {
			s.log.Error("补偿删除租户 schema 失败", zap.Uint64("tenant_id", tenantID), zap.String("schema", schema), zap.Error(err))
		}
	}
	if err := s.manager.Platform().WithContext(ctx).Unscoped().Delete(&model.Tenant{}, tenantID).Error; err != nil {
		s.log.Error("补偿删除租户记录失败", zap.Uint64("tenant_id", tenantID), zap.Error(err))
	}
	if err := s.sessions.InvalidateTenant(ctx, tenantID); err != nil {
		s.log.Warn("失效租户缓存失败", zap.Uint64("tenant_id", tenantID), zap.Error(err))
	}
}

// Update 修改租户基础资料；编码变更需重新校验唯一。
func (s *Service) Update(ctx context.Context, id uint64, in UpdateInput) (*Item, error) {
	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrTenantNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	code := strings.TrimSpace(in.Code)
	if code != t.Code {
		if err := s.ensureCodeAvailable(ctx, code, id); err != nil {
			return nil, err
		}
	}
	t.Name = in.Name
	t.Code = code
	t.ContactName = in.ContactName
	t.ContactPhone = in.ContactPhone
	res := s.manager.Platform().WithContext(ctx).Model(&model.Tenant{}).Where("id=?", id).Updates(map[string]any{"name": t.Name, "code": t.Code, "contact_name": t.ContactName, "contact_phone": t.ContactPhone})
	if res.Error != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, errcode.New(errcode.ErrTenantNotFound)
	}

	s.invalidate(ctx, id)
	items := toItems([]model.Tenant{t})
	return &items[0], nil
}

// SetStatus 启停租户。停用会同时踢掉该租户全部账号的会话，让变更立即生效。
func (s *Service) SetStatus(ctx context.Context, id uint64, status string) error {
	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.ErrTenantNotFound)
		}
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if t.Status == status {
		return nil
	}
	if err := s.manager.Platform().WithContext(ctx).Model(&model.Tenant{}).
		Where("id = ?", id).Update("status", status).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}

	s.invalidate(ctx, id)
	if status == model.StatusDisabled {
		s.revokeTenantSessions(ctx, id)
	}
	s.log.Info("租户状态已变更", zap.Uint64("tenant_id", id), zap.String("status", status))
	return nil
}

// RevokeTenantSessions 供其它模块复用：踢掉某租户下全部账号的登录态。
func (s *Service) RevokeTenantSessions(ctx context.Context, tenantID uint64) {
	s.revokeTenantSessions(ctx, tenantID)
}

// Delete 删除租户：不立即物理回收，只置为待销毁（status=deleted）并写入 PurgeAt。
//
// 宽限天数取 sys_config 的 tenant.purge_after_days（默认 7 天）。
// 待销毁期间：租户门禁按非 enabled 直接拦截、全部会话被踢下线、
// 业务键仍占用（不能重开同编码租户）；到期由系统任务 tenant_purge 物理销毁。
// 重复调用幂等：已经是待销毁态直接返回。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.ErrTenantNotFound)
		}
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if t.Status == model.StatusDeleted {
		return nil
	}

	days := s.purgeGraceDays(ctx)
	purgeAt := time.Now().AddDate(0, 0, days)
	if err := s.manager.Platform().WithContext(ctx).Model(&model.Tenant{}).Where("id = ?", id).
		Updates(map[string]any{"status": model.StatusDeleted, "purge_at": purgeAt}).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}

	s.invalidate(ctx, id)
	s.revokeTenantSessions(ctx, id)
	s.log.Info("租户已标记待销毁",
		zap.Uint64("tenant_id", id), zap.String("code", t.Code), zap.Time("purge_at", purgeAt))
	return nil
}

// Restore 撤销删除：把待销毁租户恢复为停用态并清空 PurgeAt。
// 恢复后不自动启用，需管理员手动启用，避免数据问题租户被直接放回线上。
func (s *Service) Restore(ctx context.Context, id uint64) error {
	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.ErrTenantNotFound)
		}
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if t.Status != model.StatusDeleted {
		return errcode.Newf(errcode.ErrConflict, "租户未处于待销毁状态，无需恢复")
	}

	if err := s.manager.Platform().WithContext(ctx).Model(&model.Tenant{}).Where("id = ?", id).
		Updates(map[string]any{"status": model.StatusDisabled, "purge_at": nil}).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	s.invalidate(ctx, id)
	s.log.Info("租户已撤销销毁", zap.Uint64("tenant_id", id), zap.String("code", t.Code))
	return nil
}

// RunPurge 是系统定时任务 tenant_purge 的执行体：
// 扫描到期租户后通过 purgeOne 行锁复核状态，同一事务销毁 schema 和平台侧归属记录。
//
// 审计日志 / 登录日志保留（tenant_id 作为历史溯源，不随 schema 一起删除）。
// 单租户数据库销毁失败整体回滚，存储文件进入后台清理队列；
// 已恢复或已被立即销毁的租户跳过，下次调度继续处理剩余租户。
func (s *Service) RunPurge(ctx context.Context) (string, error) {
	var due []model.Tenant
	if err := s.manager.Platform().WithContext(ctx).
		Where("status = ? AND purge_at IS NOT NULL AND purge_at <= ?", model.StatusDeleted, time.Now()).
		Find(&due).Error; err != nil {
		return "", errcode.Wrap(errcode.ErrDatabase, err)
	}
	if len(due) == 0 {
		return "没有到期待销毁的租户", nil
	}

	codes := make([]string, 0, len(due))
	for _, t := range due {
		if _, err := s.purgeOne(ctx, t.ID, nil); err != nil {
			code := errcode.From(err).Code
			if code == errcode.ErrTenantNotFound || code == errcode.ErrConflict {
				continue
			}
			return "", err
		}
		codes = append(codes, t.Code)
	}
	return fmt.Sprintf("已销毁 %d 个租户：%s", len(codes), strings.Join(codes, "、")), nil
}

// purgeGraceDays 读取宽限天数配置；参数缺失/停用/非法时回退默认 7 天。
func (s *Service) purgeGraceDays(ctx context.Context) int {
	var cfg model.Config
	err := s.manager.Platform().WithContext(ctx).
		Where("config_key = ? AND status = ?", purgeGraceDaysKey, model.StatusEnabled).
		First(&cfg).Error
	if err != nil {
		return defaultPurgeGraceDays
	}
	days, err := strconv.Atoi(strings.TrimSpace(cfg.Value))
	if err != nil || days < 0 {
		s.log.Warn("租户销毁宽限天数配置非法，回退默认值",
			zap.String("value", cfg.Value), zap.Int("default", defaultPurgeGraceDays))
		return defaultPurgeGraceDays
	}
	return days
}

// revokeTenantSessions 踢掉某租户下全部账号的登录态，best-effort：失败只告警。
func (s *Service) revokeTenantSessions(ctx context.Context, tenantID uint64) {
	var users []model.TenantUser
	if err := s.manager.Platform().WithContext(ctx).
		Select("id").Where("tenant_id = ?", tenantID).Find(&users).Error; err != nil {
		s.log.Warn("查询租户账号失败", zap.Uint64("tenant_id", tenantID), zap.Error(err))
		return
	}
	for _, u := range users {
		if err := s.sessions.RevokeUserSessions(ctx, model.UserTypeTenant, u.ID); err != nil {
			s.log.Warn("踢下线失败", zap.Uint64("user_id", u.ID), zap.Error(err))
		}
	}
}

// invalidate 失效单个租户的快照缓存，best-effort。
func (s *Service) invalidate(ctx context.Context, id uint64) {
	if err := s.sessions.InvalidateTenant(ctx, id); err != nil {
		s.log.Warn("失效租户缓存失败", zap.Uint64("tenant_id", id), zap.Error(err))
	}
}

// ensureCodeAvailable 校验租户编码唯一，excludeID 用于编辑场景排除自身。
func (s *Service) ensureCodeAvailable(ctx context.Context, code string, excludeID uint64) error {
	q := s.manager.Platform().WithContext(ctx).Model(&model.Tenant{}).Where("code = ?", code)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if n > 0 {
		return errcode.New(errcode.ErrTenantCodeTaken)
	}
	return nil
}

// toItems 把租户模型映射为返回结构。
func toItems(rows []model.Tenant) []Item {
	out := make([]Item, 0, len(rows))
	for _, r := range rows {
		out = append(out, Item{
			ID:           r.ID,
			Name:         r.Name,
			Code:         r.Code,
			ContactName:  r.ContactName,
			ContactPhone: r.ContactPhone,
			Status:       r.Status,
			ExpireAt:     r.ExpireAt,
			PurgeAt:      r.PurgeAt,
			CreatedAt:    r.CreatedAt,
		})
	}
	return out
}

// isUniqueViolation 判定 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
