// Package auth 负责登录态：登录、刷新、登出、个人信息与菜单。
//
// 登录入口只有一个：填了 tenant_code 走租户账号，没填走平台管理员。
// 这样后台可以用同一个登录页，前端按返回的 user_type 决定落地路由。
package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/middleware"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/jwtx"
	"pet-manager/server/internal/pkg/security"
)

// —— DTO ——

// LoginInput 是统一登录入参。
type LoginInput struct {
	Username   string `json:"username" binding:"required"` // 登录名
	Password   string `json:"password" binding:"required"` // 明文口令（校验时比对散列）
	TenantCode string `json:"tenant_code"`                 // 租户编码；为空表示平台管理员登录
}

// UpdateProfileInput 修改个人资料入参，目前仅允许改昵称。
type UpdateProfileInput struct {
	Nickname string `json:"nickname" binding:"required,max=64"`
}

// ChangePasswordInput 修改自己的登录口令入参。
// OldPassword 不加长度约束，避免历史短口令账号无法通过校验。
type ChangePasswordInput struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=64"`
}

// UserInfo 是前台识别身份、渲染权限的最小集合。
type UserInfo struct {
	ID          uint64   `json:"id"`          // 账号 ID
	Username    string   `json:"username"`    // 登录名
	Nickname    string   `json:"nickname"`    // 昵称
	UserType    string   `json:"user_type"`   // platform / tenant
	TenantID    uint64   `json:"tenant_id"`   // 所属租户 ID（平台账号为 0）
	TenantCode  string   `json:"tenant_code"` // 所属租户编码
	TenantName  string   `json:"tenant_name"` // 所属租户名称
	Roles       []string `json:"roles"`       // 角色编码集合
	Permissions []string `json:"permissions"` // 权限点编码集合（平台账号为通配符）
}

// LoginResult 是登录/刷新成功返回的令牌与身份信息。
type LoginResult struct {
	AccessToken  string   `json:"access_token"`  // 访问令牌
	RefreshToken string   `json:"refresh_token"` // 刷新令牌
	ExpiresIn    int64    `json:"expires_in"`    // access_token 有效期（秒）
	User         UserInfo `json:"user"`          // 登录身份信息
}

// MenuNode 是前端侧边栏节点，key 即权限点编码，便于直接对齐权限。
type MenuNode struct {
	Key      string      `json:"key"`                // 菜单唯一键，即权限点编码
	Title    string      `json:"title"`              // 菜单标题
	Path     string      `json:"path"`               // 前端路由；分组菜单为空
	Icon     string      `json:"icon,omitempty"`     // 图标名
	Children []*MenuNode `json:"children,omitempty"` // 子菜单
}

// LoginRecord 是一次登录尝试的采集数据（成功与失败都采集）。
type LoginRecord struct {
	Username   string // 尝试登录的账号名
	UserType   string // 推断的身份类型 platform / tenant
	TenantID   uint64 // 租户 ID（能解析到时）
	TenantCode string // 租户编码
	ClientIP   string // 客户端 IP
	UserAgent  string // 客户端 UA
	Status     string // model.LoginStatusSuccess / model.LoginStatusFail
	Message    string // 失败原因（成功时为空）
}

// LoginRecorder 由消费方（auth）定义、由 loginlog 模块实现，
// 避免 auth 反向依赖日志模块，也让本模块可测试时替换掉落库实现。
type LoginRecorder interface {
	Record(ctx context.Context, r LoginRecord)
}

// —— Service ——

// Service 实现登录态相关用例，同时作为 router.Module 注册认证路由。
type Service struct {
	manager  *db.Manager                   // 平台库连接管理器
	sessions *session.Store                // 会话与权限缓存
	jwt      *jwtx.Manager                 // 令牌签发/解析
	perms    middleware.PermissionResolver // 权限点解析器（由 rbac 模块实现）
	recorder LoginRecorder                 // 登录尝试采集器（可为 nil）
	log      *zap.Logger                   // 日志
}

// NewService 组装 auth 服务。
func NewService(manager *db.Manager, sessions *session.Store, jwtMgr *jwtx.Manager, perms middleware.PermissionResolver, recorder LoginRecorder, log *zap.Logger) *Service {
	return &Service{manager: manager, sessions: sessions, jwt: jwtMgr, perms: perms, recorder: recorder, log: log}
}

// Login 统一登录入口。
func (s *Service) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	if code := strings.TrimSpace(in.TenantCode); code != "" {
		return s.loginTenant(ctx, code, in)
	}
	return s.loginPlatform(ctx, in)
}

// loginPlatform 平台管理员登录：校验平台账号后签发通配权限令牌。
func (s *Service) loginPlatform(ctx context.Context, in LoginInput) (*LoginResult, error) {
	var u model.PlatformUser
	err := s.manager.Platform().WithContext(ctx).Where("username = ?", in.Username).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrLoginFailed)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if u.Status != model.StatusEnabled {
		return nil, errcode.New(errcode.ErrAccountDisabled)
	}
	if !security.VerifyPassword(u.Password, in.Password) {
		return nil, errcode.New(errcode.ErrLoginFailed)
	}

	user := UserInfo{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    u.Nickname,
		UserType:    model.UserTypePlatform,
		Roles:       []string{"platform_admin"},
		Permissions: []string{middleware.PermissionWildcard},
	}
	claims := jwtx.Claims{UserID: u.ID, Username: u.Username, UserType: model.UserTypePlatform}
	return s.issue(ctx, claims, user)
}

// loginTenant 租户子账号登录：先过租户门禁（启用/未到期）再校验账号，
// 成功后解析角色与权限点一并写入令牌与会话。
func (s *Service) loginTenant(ctx context.Context, code string, in LoginInput) (*LoginResult, error) {
	t, err := s.findTenant(ctx, code)
	if err != nil {
		return nil, err
	}

	var u model.TenantUser
	err = s.manager.Platform().WithContext(ctx).
		Where("tenant_id = ? AND username = ?", t.ID, in.Username).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrLoginFailed)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if u.Status != model.StatusEnabled {
		return nil, errcode.New(errcode.ErrAccountDisabled)
	}
	if !security.VerifyPassword(u.Password, in.Password) {
		return nil, errcode.New(errcode.ErrLoginFailed)
	}

	user, err := s.tenantUserInfo(ctx, t, u)
	if err != nil {
		return nil, err
	}
	claims := jwtx.Claims{
		UserID:       u.ID,
		Username:     u.Username,
		UserType:     model.UserTypeTenant,
		TenantID:     t.ID,
		TenantCode:   t.Code,
		TenantSchema: t.SchemaName,
	}
	return s.issue(ctx, claims, *user)
}

// Refresh 用 refresh 凭证换一对新凭证。
//
// 这里刻意不复用旧 sid，而是沿用同一个 sid 重新续期：sid 不变意味着"同一次登录"，
// 便于后续在会话里挂更多状态；同时重新 SaveSession 会把会话 TTL 顺延到新的 refresh 有效期。
func (s *Service) Refresh(ctx context.Context, raw string) (*LoginResult, error) {
	claims, err := s.jwt.Parse(raw)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != jwtx.TokenTypeRefresh {
		return nil, errcode.New(errcode.ErrTokenInvalid)
	}
	alive, err := s.sessions.SessionAlive(ctx, claims.UserType, claims.UserID, claims.SessionID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}
	if !alive {
		return nil, errcode.New(errcode.ErrSessionRevoked)
	}

	user, fresh, err := s.revalidate(ctx, claims)
	if err != nil {
		return nil, err
	}
	fresh.SessionID = claims.SessionID
	return s.issue(ctx, fresh, *user)
}

// Logout 注销当前会话。
func (s *Service) Logout(ctx context.Context) error {
	userType := ctxkey.UserType(ctx)
	userID := ctxkey.UserID(ctx)
	sid := ctxkey.SessionID(ctx)
	if userType == "" || userID == 0 || sid == "" {
		return errcode.New(errcode.ErrUnauthorized)
	}
	if err := s.sessions.RevokeSession(ctx, userType, userID, sid); err != nil {
		return errcode.Wrap(errcode.ErrInternal, err)
	}
	return nil
}

// Profile 返回当前登录账号信息（含最新权限点）。
func (s *Service) Profile(ctx context.Context) (*UserInfo, error) {
	switch ctxkey.UserType(ctx) {
	case ctxkey.UserTypePlatform:
		return s.platformUserInfo(ctx, ctxkey.UserID(ctx))
	case ctxkey.UserTypeTenant:
		t, err := s.findTenantByID(ctx, ctxkey.TenantID(ctx))
		if err != nil {
			return nil, err
		}
		var u model.TenantUser
		if err := s.manager.Platform().WithContext(ctx).
			Where("id = ? AND tenant_id = ?", ctxkey.UserID(ctx), t.ID).First(&u).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errcode.New(errcode.ErrUnauthorized)
			}
			return nil, errcode.Wrap(errcode.ErrDatabase, err)
		}
		return s.tenantUserInfo(ctx, t, u)
	default:
		return nil, errcode.New(errcode.ErrUnauthorized)
	}
}

// UpdateProfile 修改当前登录账号的昵称，返回最新的 UserInfo（含权限点）。
// 平台/租户账号表都在平台库，租户账号这里不走租户事务（不涉及租户 schema）。
func (s *Service) UpdateProfile(ctx context.Context, in UpdateProfileInput) (*UserInfo, error) {
	nickname := strings.TrimSpace(in.Nickname)
	if nickname == "" {
		return nil, errcode.New(errcode.ErrInvalidParam)
	}

	switch ctxkey.UserType(ctx) {
	case ctxkey.UserTypePlatform:
		id := ctxkey.UserID(ctx)
		res := s.manager.Platform().WithContext(ctx).Model(&model.PlatformUser{}).
			Where("id = ?", id).Update("nickname", nickname)
		if res.Error != nil {
			return nil, errcode.Wrap(errcode.ErrDatabase, res.Error)
		}
		if res.RowsAffected == 0 {
			return nil, errcode.New(errcode.ErrUnauthorized)
		}
		return s.platformUserInfo(ctx, id)

	case ctxkey.UserTypeTenant:
		t, err := s.findTenantByID(ctx, ctxkey.TenantID(ctx))
		if err != nil {
			return nil, err
		}
		id := ctxkey.UserID(ctx)
		res := s.manager.Platform().WithContext(ctx).Model(&model.TenantUser{}).
			Where("id = ? AND tenant_id = ?", id, t.ID).Update("nickname", nickname)
		if res.Error != nil {
			return nil, errcode.Wrap(errcode.ErrDatabase, res.Error)
		}
		if res.RowsAffected == 0 {
			return nil, errcode.New(errcode.ErrUnauthorized)
		}
		var u model.TenantUser
		if err := s.manager.Platform().WithContext(ctx).
			Where("id = ? AND tenant_id = ?", id, t.ID).First(&u).Error; err != nil {
			return nil, errcode.Wrap(errcode.ErrDatabase, err)
		}
		return s.tenantUserInfo(ctx, t, u)

	default:
		return nil, errcode.New(errcode.ErrUnauthorized)
	}
}

// ChangePassword 修改当前登录账号的口令。
// 校验原口令通过后写入新散列，并踢掉该账号的全部会话（含当前会话），
// 前端应引导用户用新口令重新登录。
func (s *Service) ChangePassword(ctx context.Context, in ChangePasswordInput) error {
	if in.OldPassword == in.NewPassword {
		return errcode.Newf(errcode.ErrInvalidParam, "新密码不能与原密码相同")
	}
	hashed, err := security.HashPassword(in.NewPassword)
	if err != nil {
		return errcode.Wrap(errcode.ErrInternal, err)
	}

	switch ctxkey.UserType(ctx) {
	case ctxkey.UserTypePlatform:
		id := ctxkey.UserID(ctx)
		var u model.PlatformUser
		if err := s.manager.Platform().WithContext(ctx).First(&u, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errcode.New(errcode.ErrUnauthorized)
			}
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if !security.VerifyPassword(u.Password, in.OldPassword) {
			return errcode.Newf(errcode.ErrInvalidParam, "原密码不正确")
		}
		if err := s.manager.Platform().WithContext(ctx).Model(&model.PlatformUser{}).
			Where("id = ?", id).Update("password", hashed).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		s.revokeSessions(ctx, ctxkey.UserTypePlatform, id)
		return nil

	case ctxkey.UserTypeTenant:
		t, err := s.findTenantByID(ctx, ctxkey.TenantID(ctx))
		if err != nil {
			return err
		}
		id := ctxkey.UserID(ctx)
		var u model.TenantUser
		if err := s.manager.Platform().WithContext(ctx).
			Where("id = ? AND tenant_id = ?", id, t.ID).First(&u).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errcode.New(errcode.ErrUnauthorized)
			}
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if !security.VerifyPassword(u.Password, in.OldPassword) {
			return errcode.Newf(errcode.ErrInvalidParam, "原密码不正确")
		}
		if err := s.manager.Platform().WithContext(ctx).Model(&model.TenantUser{}).
			Where("id = ? AND tenant_id = ?", id, t.ID).Update("password", hashed).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		s.revokeSessions(ctx, ctxkey.UserTypeTenant, id)
		return nil

	default:
		return errcode.New(errcode.ErrUnauthorized)
	}
}

// revokeSessions 改密后踢掉账号全部会话，best-effort：失败只告警不阻断主流程。
func (s *Service) revokeSessions(ctx context.Context, userType string, userID uint64) {
	if err := s.sessions.RevokeUserSessions(ctx, userType, userID); err != nil {
		s.log.Warn("改密后踢下线失败",
			zap.String("user_type", userType), zap.Uint64("user_id", userID), zap.Error(err))
	}
}

// Menus 返回当前账号可见的菜单树（type=menu 的权限点）。
func (s *Service) Menus(ctx context.Context) ([]*MenuNode, error) {
	scope := ctxkey.UserType(ctx)
	if scope != ctxkey.UserTypePlatform && scope != ctxkey.UserTypeTenant {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	user, err := s.Profile(ctx)
	if err != nil {
		return nil, err
	}

	var list []model.Permission
	if err := s.manager.Platform().WithContext(ctx).
		Where("scope = ? AND type = ?", scope, model.PermTypeMenu).
		Order("sort asc, id asc").Find(&list).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	visible := make([]model.Permission, 0, len(list))
	for _, p := range list {
		if middleware.HasAny(user.Permissions, []string{p.Code}) {
			visible = append(visible, p)
		}
	}
	return pruneEmptyMenus(buildMenuTree(visible)), nil
}

// —— 内部方法 ——

// issue 建会话并签发一对凭证。
func (s *Service) issue(ctx context.Context, claims jwtx.Claims, user UserInfo) (*LoginResult, error) {
	if claims.SessionID == "" {
		claims.SessionID = security.RandomToken(16)
	}
	if err := s.sessions.SaveSession(ctx, claims.UserType, claims.UserID, claims.SessionID, s.jwt.RefreshTTL()); err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}

	access, expiresIn, err := s.jwt.Sign(claims, jwtx.TokenTypeAccess)
	if err != nil {
		return nil, err
	}
	refresh, _, err := s.jwt.Sign(claims, jwtx.TokenTypeRefresh)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    expiresIn,
		User:         user,
	}, nil
}

// revalidate 在刷新凭证时重新校验账号与租户状态，并刷新权限点。
// 返回最新的 UserInfo 与用于签发的 Claims 骨架。
func (s *Service) revalidate(ctx context.Context, old *jwtx.Claims) (*UserInfo, jwtx.Claims, error) {
	switch old.UserType {
	case ctxkey.UserTypePlatform:
		user, err := s.platformUserInfo(ctx, old.UserID)
		if err != nil {
			return nil, jwtx.Claims{}, err
		}
		return user, jwtx.Claims{UserID: old.UserID, Username: old.Username, UserType: old.UserType}, nil

	case ctxkey.UserTypeTenant:
		// 租户可能被停用/到期，刷新时一并拦掉，避免拿着旧凭证继续续期
		t, err := s.findTenant(ctx, old.TenantCode)
		if err != nil {
			return nil, jwtx.Claims{}, err
		}
		var u model.TenantUser
		if err := s.manager.Platform().WithContext(ctx).
			Where("id = ? AND tenant_id = ?", old.UserID, t.ID).First(&u).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, jwtx.Claims{}, errcode.New(errcode.ErrSessionRevoked)
			}
			return nil, jwtx.Claims{}, errcode.Wrap(errcode.ErrDatabase, err)
		}
		if u.Status != model.StatusEnabled {
			return nil, jwtx.Claims{}, errcode.New(errcode.ErrAccountDisabled)
		}
		user, err := s.tenantUserInfo(ctx, t, u)
		if err != nil {
			return nil, jwtx.Claims{}, err
		}
		claims := jwtx.Claims{
			UserID:       u.ID,
			Username:     u.Username,
			UserType:     model.UserTypeTenant,
			TenantID:     t.ID,
			TenantCode:   t.Code,
			TenantSchema: t.SchemaName,
		}
		return user, claims, nil

	default:
		return nil, jwtx.Claims{}, errcode.New(errcode.ErrTokenInvalid)
	}
}

// platformUserInfo 组装平台账号的 UserInfo，平台账号固定视为全量权限。
func (s *Service) platformUserInfo(ctx context.Context, id uint64) (*UserInfo, error) {
	var u model.PlatformUser
	if err := s.manager.Platform().WithContext(ctx).First(&u, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrUnauthorized)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if u.Status != model.StatusEnabled {
		return nil, errcode.New(errcode.ErrAccountDisabled)
	}
	return &UserInfo{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    u.Nickname,
		UserType:    model.UserTypePlatform,
		Roles:       []string{"platform_admin"},
		Permissions: []string{middleware.PermissionWildcard},
	}, nil
}

// tenantUserInfo 组装租户账号的 UserInfo。
// 权限点由 PermissionResolver（rbac）在 TenantTx 内解析并写入会话缓存；
// 角色编码同事务内从租户 schema 的 t_user_role/t_role 读取。
func (s *Service) tenantUserInfo(ctx context.Context, t *model.Tenant, u model.TenantUser) (*UserInfo, error) {
	perms, err := s.perms.ResolvePermissions(ctx, t.ID, u.ID, t.SchemaName)
	if err != nil {
		return nil, err
	}
	if err := s.sessions.CachePermissions(ctx, t.ID, u.ID, perms); err != nil {
		s.log.Warn("缓存权限点失败", zap.Uint64("tenant_id", t.ID), zap.Uint64("user_id", u.ID), zap.Error(err))
	}

	roles := make([]string, 0, 2)
	if err := s.manager.TenantTx(ctx, t.SchemaName, func(tx *gorm.DB) error {
		return tx.Raw(`SELECT r.code FROM t_user_role ur
			JOIN t_role r ON r.id = ur.role_id
			WHERE ur.user_id = ? ORDER BY r.id`, u.ID).Scan(&roles).Error
	}); err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	if roles == nil {
		roles = []string{}
	}
	if perms == nil {
		perms = []string{}
	}

	return &UserInfo{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    u.Nickname,
		UserType:    model.UserTypeTenant,
		TenantID:    t.ID,
		TenantCode:  t.Code,
		TenantName:  t.Name,
		Roles:       roles,
		Permissions: perms,
	}, nil
}

// findTenant 按编码取租户，并做启用/到期门禁。
func (s *Service) findTenant(ctx context.Context, code string) (*model.Tenant, error) {
	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).Where("code = ?", code).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrTenantNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return &t, checkTenantUsable(&t)
}

// findTenantByID 按 ID 取租户，并做启用/到期门禁。
func (s *Service) findTenantByID(ctx context.Context, id uint64) (*model.Tenant, error) {
	var t model.Tenant
	if err := s.manager.Platform().WithContext(ctx).First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrTenantNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return &t, checkTenantUsable(&t)
}

// checkTenantUsable 是登录/刷新/取个人信息时的租户门禁。
func checkTenantUsable(t *model.Tenant) error {
	if t.Status != model.StatusEnabled {
		return errcode.New(errcode.ErrTenantDisabled)
	}
	if t.ExpireAt != nil && t.ExpireAt.Before(time.Now()) {
		return errcode.New(errcode.ErrTenantExpired)
	}
	return nil
}

// buildMenuTree 把扁平权限点列表按 ParentID 组装成菜单树。
func buildMenuTree(list []model.Permission) []*MenuNode {
	nodes := make(map[uint64]*MenuNode, len(list))
	for _, p := range list {
		nodes[p.ID] = &MenuNode{Key: p.Code, Title: p.Name, Path: p.Path, Icon: p.Icon}
	}
	roots := make([]*MenuNode, 0, len(list))
	for _, p := range list {
		node := nodes[p.ID]
		parent, ok := nodes[p.ParentID]
		if !ok {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}

// pruneEmptyMenus 递归剔除"无路由且无子节点"的菜单节点。
//
// 父分组菜单（如"日志管理""配置管理"）本身不带路由，仅作分组；
// 其子项若因权限过滤后为空，父节点会退化成可点击的空叶，这里剪掉。
func pruneEmptyMenus(nodes []*MenuNode) []*MenuNode {
	out := make([]*MenuNode, 0, len(nodes))
	for _, n := range nodes {
		n.Children = pruneEmptyMenus(n.Children)
		if n.Path == "" && len(n.Children) == 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}
