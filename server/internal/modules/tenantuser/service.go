// Package tenantuser 承载租户端的子账号管理。
//
// 这里有个值得说明的设计点：账号表在平台库，角色绑定表在租户 schema，
// 但请求进入 /tenant/* 时 TenantScope 已经开启了事务并把 search_path 设为
// "tenant_x, public"，所以同一个事务里既能写 t_user_role 也能写 sys_tenant_user。
// 于是"建账号 + 绑角色"天然是一个原子操作，不需要跨库补偿。
package tenantuser

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/pkg/security"
	"pet-manager/server/internal/router"
)

// Item 与后台契约一致，role_names 便于列表直接展示。
type Item struct {
	ID        uint64    `json:"id"`         // 账号 ID
	Username  string    `json:"username"`   // 登录名
	Nickname  string    `json:"nickname"`   // 昵称
	Status    string    `json:"status"`     // enabled / disabled
	RoleIDs   []uint64  `json:"role_ids"`   // 绑定的角色 ID
	RoleNames []string  `json:"role_names"` // 绑定的角色名
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// ListQuery 是子账号列表查询条件。
type ListQuery struct {
	pagination.Query
	Keyword string `form:"keyword"` // 用户名/昵称关键字
}

// CreateInput 是新建子账号入参。
type CreateInput struct {
	Username string   `json:"username" binding:"required,min=3,max=64"` // 登录名
	Password string   `json:"password" binding:"required,min=6,max=64"` // 初始口令
	Nickname string   `json:"nickname" binding:"max=64"`                // 昵称
	RoleIDs  []uint64 `json:"role_ids"`                                 // 角色绑定
}

// UpdateInput 是编辑子账号入参。
type UpdateInput struct {
	Nickname string   `json:"nickname" binding:"max=64"`                         // 昵称
	Status   string   `json:"status" binding:"omitempty,oneof=enabled disabled"` // 状态
	RoleIDs  []uint64 `json:"role_ids"`                                          // 角色绑定
}

// ResetPasswordInput 是重置子账号口令入参。
type ResetPasswordInput struct {
	Password string `json:"password" binding:"required,min=6,max=64"` // 新口令
}

// Service 承载租户端子账号管理，账号与角色绑定复用租户事务保持原子。
type Service struct {
	sessions *session.Store // 会话与权限缓存
	log      *zap.Logger    // 日志
}

// NewService 组装子账号服务。
func NewService(sessions *session.Store, log *zap.Logger) *Service {
	return &Service{sessions: sessions, log: log}
}

// Name 实现 router.Module。
func (s *Service) Name() string { return "tenant.user" }

// Register 挂载租户端子账号接口；Tenant 分组已内置登录与租户门禁。
func (s *Service) Register(r *router.Router) {
	g := r.Tenant("/users")
	g.GET("", r.Require("tenant.user.list"), s.handleList)
	g.POST("", r.Require("tenant.user.create"), s.handleCreate)
	g.PUT("/:id", r.Require("tenant.user.update"), s.handleUpdate)
	g.DELETE("/:id", r.Require("tenant.user.delete"), s.handleDelete)
	g.POST("/:id/reset-password", r.Require("tenant.user.reset"), s.handleResetPassword)
}

// List 分页查询当前租户子账号，附带角色绑定（批量查询避免 N+1）。
func (s *Service) List(ctx context.Context, q ListQuery) (*response.Page[Item], error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := ctxkey.TenantID(ctx)

	base := tx.Model(&model.TenantUser{}).Where("tenant_id = ?", tenantID)
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		like := "%" + kw + "%"
		base = base.Where("username ILIKE ? OR nickname ILIKE ?", like, like)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	var users []model.TenantUser
	if err := base.Order("id asc").Offset(q.Offset()).Limit(q.Limit()).Find(&users).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if len(users) == 0 {
		return response.NewPage([]Item{}, total), nil
	}

	ids := make([]uint64, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	roleMap, err := s.rolesOfUsers(tx, ids)
	if err != nil {
		return nil, err
	}

	items := make([]Item, 0, len(users))
	for _, u := range users {
		bind := roleMap[u.ID]
		items = append(items, Item{
			ID:        u.ID,
			Username:  u.Username,
			Nickname:  u.Nickname,
			Status:    u.Status,
			RoleIDs:   bind.ids,
			RoleNames: bind.names,
			CreatedAt: u.CreatedAt,
		})
	}
	return response.NewPage(items, total), nil
}

// Create 在当前租户事务内创建子账号并绑定角色（两者原子完成）。
func (s *Service) Create(ctx context.Context, in CreateInput) (*Item, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := ctxkey.TenantID(ctx)

	if err := s.ensureUsernameAvailable(tx, tenantID, in.Username, 0); err != nil {
		return nil, err
	}
	roleIDs, err := s.validRoleIDs(tx, in.RoleIDs)
	if err != nil {
		return nil, err
	}

	hashed, err := security.HashPassword(in.Password)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}

	u := model.TenantUser{
		TenantID: tenantID,
		Username: strings.TrimSpace(in.Username),
		Password: hashed,
		Nickname: in.Nickname,
		Status:   model.StatusEnabled,
	}
	if err := tx.Create(&u).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if err := s.replaceRoles(tx, u.ID, roleIDs); err != nil {
		return nil, err
	}

	roleMap, err := s.rolesOfUsers(tx, []uint64{u.ID})
	if err != nil {
		return nil, err
	}
	bind := roleMap[u.ID]
	return &Item{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  u.Nickname,
		Status:    u.Status,
		RoleIDs:   bind.ids,
		RoleNames: bind.names,
		CreatedAt: u.CreatedAt,
	}, nil
}

// Update 修改子账号昵称/状态/角色绑定；主账号不可停用，变更后失效权限缓存并视情况踢下线。
func (s *Service) Update(ctx context.Context, id uint64, in UpdateInput) error {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return err
	}
	tenantID := ctxkey.TenantID(ctx)

	u, err := s.load(tx, tenantID, id)
	if err != nil {
		return err
	}

	status := in.Status
	if status == "" {
		status = u.Status
	}
	// 主账号是租户的唯一兜底入口，不允许被停用，否则租户会把自己锁在门外
	if u.IsOwner && status != model.StatusEnabled {
		return errcode.New(errcode.ErrOwnerProtected)
	}

	roleIDs, err := s.validRoleIDs(tx, in.RoleIDs)
	if err != nil {
		return err
	}

	u.Nickname = in.Nickname
	u.Status = status
	if err := tx.Save(&u).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if err := s.replaceRoles(tx, u.ID, roleIDs); err != nil {
		return err
	}

	// 角色或状态变了：权限点缓存失效，且停用要立刻踢下线
	s.invalidatePermissions(ctx, tenantID)
	if status != model.StatusEnabled {
		s.revokeSessions(ctx, u.ID)
	}
	return nil
}

// Delete 软删子账号并解除角色绑定；禁止删除自己与主账号。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return err
	}
	tenantID := ctxkey.TenantID(ctx)

	if id == ctxkey.UserID(ctx) {
		return errcode.New(errcode.ErrCannotDeleteSelf)
	}
	u, err := s.load(tx, tenantID, id)
	if err != nil {
		return err
	}
	if u.IsOwner {
		return errcode.New(errcode.ErrOwnerProtected)
	}

	if err := tx.Where("user_id = ?", u.ID).Delete(&model.UserRole{}).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if err := tx.Delete(&model.TenantUser{}, u.ID).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}

	s.invalidatePermissions(ctx, tenantID)
	s.revokeSessions(ctx, u.ID)
	return nil
}

// ResetPassword 重置子账号口令，成功后踢掉该账号全部会话。
func (s *Service) ResetPassword(ctx context.Context, id uint64, in ResetPasswordInput) error {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return err
	}
	tenantID := ctxkey.TenantID(ctx)

	u, err := s.load(tx, tenantID, id)
	if err != nil {
		return err
	}
	hashed, err := security.HashPassword(in.Password)
	if err != nil {
		return errcode.Wrap(errcode.ErrInternal, err)
	}
	if err := tx.Model(&model.TenantUser{}).Where("id = ?", u.ID).
		Update("password", hashed).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	// 改密后旧会话立即失效，避免旧口令泄漏后长期可用
	s.revokeSessions(ctx, u.ID)
	return nil
}

// —— 内部方法 ——

// load 按 ID + 租户 ID 读取账号，确保不会跨租户访问。
func (s *Service) load(tx *gorm.DB, tenantID, id uint64) (model.TenantUser, error) {
	var u model.TenantUser
	err := tx.Where("id = ? AND tenant_id = ?", id, tenantID).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return u, errcode.New(errcode.ErrNotFound)
		}
		return u, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return u, nil
}

// ensureUsernameAvailable 校验同一租户内登录名唯一，excludeID 用于编辑场景排除自身。
func (s *Service) ensureUsernameAvailable(tx *gorm.DB, tenantID uint64, username string, excludeID uint64) error {
	q := tx.Model(&model.TenantUser{}).Where("tenant_id = ? AND username = ?", tenantID, strings.TrimSpace(username))
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if n > 0 {
		return errcode.New(errcode.ErrUsernameTaken)
	}
	return nil
}

// validRoleIDs 过滤出确实属于当前租户的角色，防止越权绑定到别的租户角色。
func (s *Service) validRoleIDs(tx *gorm.DB, ids []uint64) ([]uint64, error) {
	uniq := make([]uint64, 0, len(ids))
	seen := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return nil, nil
	}

	var found []uint64
	if err := tx.Model(&model.Role{}).Where("id IN ?", uniq).Pluck("id", &found).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if len(found) != len(uniq) {
		return nil, errcode.New(errcode.ErrNotFound)
	}
	return uniq, nil
}

// replaceRoles 全量重写账号的角色绑定：先清空再写入。
func (s *Service) replaceRoles(tx *gorm.DB, userID uint64, roleIDs []uint64) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]model.UserRole, 0, len(roleIDs))
	for _, rid := range roleIDs {
		rows = append(rows, model.UserRole{UserID: userID, RoleID: rid})
	}
	if err := tx.Create(&rows).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	return nil
}

// roleBinding 是单个账号的角色绑定视图（ID 与名称一一对应）。
type roleBinding struct {
	ids   []uint64
	names []string
}

// rolesOfUsers 一次查询取回多个账号的角色绑定，避免 N+1。
func (s *Service) rolesOfUsers(tx *gorm.DB, userIDs []uint64) (map[uint64]roleBinding, error) {
	out := make(map[uint64]roleBinding, len(userIDs))

	var bindings []model.UserRole
	if err := tx.Where("user_id IN ?", userIDs).Find(&bindings).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if len(bindings) == 0 {
		return out, nil
	}

	roleIDs := make([]uint64, 0, len(bindings))
	for _, b := range bindings {
		roleIDs = append(roleIDs, b.RoleID)
	}
	var roles []model.Role
	if err := tx.Where("id IN ?", roleIDs).Find(&roles).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	nameOf := make(map[uint64]string, len(roles))
	for _, r := range roles {
		nameOf[r.ID] = r.Name
	}

	for _, b := range bindings {
		cur := out[b.UserID]
		cur.ids = append(cur.ids, b.RoleID)
		if n, ok := nameOf[b.RoleID]; ok {
			cur.names = append(cur.names, n)
		}
		out[b.UserID] = cur
	}
	for k, v := range out {
		if v.ids == nil {
			v.ids = []uint64{}
		}
		if v.names == nil {
			v.names = []string{}
		}
		out[k] = v
	}
	return out, nil
}

// invalidatePermissions 失效该租户全部账号的权限缓存，best-effort。
func (s *Service) invalidatePermissions(ctx context.Context, tenantID uint64) {
	if err := s.sessions.InvalidateTenantPermissions(ctx, tenantID); err != nil {
		s.log.Warn("失效权限缓存失败", zap.Uint64("tenant_id", tenantID), zap.Error(err))
	}
}

// revokeSessions 踢掉指定租户账号的全部会话，best-effort。
func (s *Service) revokeSessions(ctx context.Context, userID uint64) {
	if err := s.sessions.RevokeUserSessions(ctx, model.UserTypeTenant, userID); err != nil {
		s.log.Warn("踢下线失败", zap.Uint64("user_id", userID), zap.Error(err))
	}
}
