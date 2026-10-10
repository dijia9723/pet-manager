package auth

import (
	"strings"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "auth" }

// Register 挂载认证接口。
//   - Public：登录、刷新，未登录可达；
//   - Authed：登出、个人信息、菜单，只要求"凭证有效"，不区分平台/租户身份。
func (s *Service) Register(r *router.Router) {
	pub := r.Public("/auth")
	pub.POST("/login", s.handleLogin)
	pub.POST("/refresh", s.handleRefresh)

	authed := r.Authed("/auth")
	authed.POST("/logout", s.handleLogout)
	authed.GET("/profile", s.handleProfile)
	authed.PUT("/profile", s.handleUpdateProfile)
	authed.POST("/change-password", s.handleChangePassword)
	authed.GET("/menus", s.handleMenus)
}

// handleLogin 平台/租户统一登录
//
//	@Summary		统一登录
//	@Description	携带 tenant_code 时按租户子账号登录，不携带时按平台管理员登录；成功返回双令牌与身份信息
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			body	body		LoginInput		true	"登录参数"
//	@Success		200		{object}	response.Body	"data 为 LoginResult"
//	@Failure		401		{object}	response.Body	"用户名或密码错误 / 账号停用"
//	@Router			/auth/login [post]
func (s *Service) handleLogin(c *gin.Context) {
	var in LoginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.New(errcode.ErrInvalidParam))
		return
	}
	result, err := s.Login(c.Request.Context(), in)
	s.recordLogin(c, in, result, err)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// RefreshInput 刷新凭证入参。
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"` // 登录时下发的刷新令牌
}

// recordLogin 采集一次登录尝试（成功与失败都记），best-effort：写入失败不影响登录响应。
func (s *Service) recordLogin(c *gin.Context, in LoginInput, result *LoginResult, loginErr error) {
	if s.recorder == nil {
		return
	}
	rec := LoginRecord{
		Username:   strings.TrimSpace(in.Username),
		TenantCode: strings.TrimSpace(in.TenantCode),
		ClientIP:   c.ClientIP(),
		UserAgent:  c.Request.UserAgent(),
		Status:     model.LoginStatusSuccess,
	}
	if loginErr != nil {
		rec.Status = model.LoginStatusFail
		rec.Message = errcode.From(loginErr).Msg
		// 失败时拿不到账号信息，按是否携带租户编码推断身份
		if rec.TenantCode != "" {
			rec.UserType = model.UserTypeTenant
		} else {
			rec.UserType = model.UserTypePlatform
		}
	} else if result != nil {
		rec.UserType = result.User.UserType
		rec.TenantID = result.User.TenantID
		if result.User.TenantCode != "" {
			rec.TenantCode = result.User.TenantCode
		}
	}
	s.recorder.Record(c.Request.Context(), rec)
}

// handleRefresh 刷新访问凭证
//
//	@Summary		刷新令牌
//	@Description	用 refresh_token 换取新的双令牌，旧会话保持有效
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Param			body	body		RefreshInput	true	"刷新参数"
//	@Success		200		{object}	response.Body	"data 为 LoginResult"
//	@Failure		401		{object}	response.Body	"刷新凭证无效或已过期"
//	@Router			/auth/refresh [post]
func (s *Service) handleRefresh(c *gin.Context) {
	var in RefreshInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.New(errcode.ErrInvalidParam))
		return
	}
	result, err := s.Refresh(c.Request.Context(), in.RefreshToken)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// handleLogout 登出并注销当前会话
//
//	@Summary		登出
//	@Description	注销当前 access_token 对应的会话，令牌立即失效（可强制下线）
//	@Tags			认证
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Body
//	@Failure		401	{object}	response.Body
//	@Router			/auth/logout [post]
func (s *Service) handleLogout(c *gin.Context) {
	if err := s.Logout(c.Request.Context()); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleProfile 当前登录账号信息
//
//	@Summary		当前账号信息
//	@Description	返回当前登录账号的身份、角色编码与权限点集合，不区分平台/租户身份
//	@Tags			认证
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Body	"data 为 UserInfo"
//	@Failure		401	{object}	response.Body
//	@Router			/auth/profile [get]
func (s *Service) handleProfile(c *gin.Context) {
	user, err := s.Profile(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}

// handleUpdateProfile 修改当前账号资料
//
//	@Summary		修改个人资料
//	@Description	修改当前登录账号的昵称（平台/租户账号通用），返回最新 UserInfo
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		UpdateProfileInput	true	"个人资料"
//	@Success		200		{object}	response.Body		"data 为 UserInfo"
//	@Failure		400		{object}	response.Body		"参数不合法"
//	@Router			/auth/profile [put]
func (s *Service) handleUpdateProfile(c *gin.Context) {
	var in UpdateProfileInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	user, err := s.UpdateProfile(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}

// handleChangePassword 修改当前账号密码
//
//	@Summary		修改密码
//	@Description	校验原密码后写入新密码，成功后该账号全部会话立即失效，需用新密码重新登录
//	@Tags			认证
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		ChangePasswordInput	true	"改密参数"
//	@Success		200		{object}	response.Body
//	@Failure		400		{object}	response.Body	"原密码不正确 / 新密码不合规"
//	@Router			/auth/change-password [post]
func (s *Service) handleChangePassword(c *gin.Context) {
	var in ChangePasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	if err := s.ChangePassword(c.Request.Context(), in); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleMenus 当前账号可见的菜单树
//
//	@Summary		我的菜单
//	@Description	按账号权限点返回侧边栏菜单树，key 即权限点编码
//	@Tags			认证
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Body	"data 为 []MenuNode"
//	@Failure		401	{object}	response.Body
//	@Router			/auth/menus [get]
func (s *Service) handleMenus(c *gin.Context) {
	menus, err := s.Menus(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, menus)
}
