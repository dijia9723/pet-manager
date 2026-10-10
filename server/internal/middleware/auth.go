package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/jwtx"
	"pet-manager/server/internal/pkg/response"
)

// PermissionResolver 由 RBAC 模块实现，中间件只依赖接口，避免反向依赖。
type PermissionResolver interface {
	ResolvePermissions(ctx context.Context, tenantID, userID uint64, schema string) ([]string, error)
}

// TenantResolver 由租户模块实现，返回带缓存的租户快照。
type TenantResolver interface {
	TenantByID(ctx context.Context, id uint64) (*session.TenantInfo, error)
}

// Authenticator 负责凭证解析与会话校验。
type Authenticator struct {
	jwt      *jwtx.Manager  // 凭证签发/校验
	sessions *session.Store // 会话存活校验
}

// NewAuthenticator 构造鉴权器。
func NewAuthenticator(jwtMgr *jwtx.Manager, sessions *session.Store) *Authenticator {
	return &Authenticator{jwt: jwtMgr, sessions: sessions}
}

// Require 要求请求携带有效凭证，且用户类型匹配（platform / tenant）。
//
// 解析成功后写入上下文：user_id / username / user_type，
// 租户身份额外写入 tenant_id / tenant_code / tenant_schema。
func (a *Authenticator) Require(userType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			response.Abort(c, errcode.New(errcode.ErrUnauthorized))
			return
		}

		claims, err := a.jwt.Parse(token)
		if err != nil {
			response.Abort(c, err)
			return
		}
		if claims.TokenType != jwtx.TokenTypeAccess {
			response.Abort(c, errcode.New(errcode.ErrTokenInvalid))
			return
		}
		if userType != "" && claims.UserType != userType {
			response.Abort(c, errcode.New(errcode.ErrForbiddenAction))
			return
		}

		// 会话校验让"注销 / 踢下线 / 停用账号"能够立即生效
		alive, err := a.sessions.SessionAlive(c.Request.Context(), claims.UserType, claims.UserID, claims.SessionID)
		if err != nil {
			response.Abort(c, errcode.Wrap(errcode.ErrInternal, err))
			return
		}
		if !alive {
			response.Abort(c, errcode.New(errcode.ErrSessionRevoked))
			return
		}

		ctx := ctxkey.WithUser(c.Request.Context(), claims.UserID, claims.Username, claims.UserType)
		ctx = ctxkey.WithSession(ctx, claims.SessionID)
		if claims.UserType == ctxkey.UserTypeTenant {
			ctx = ctxkey.WithTenant(ctx, claims.TenantID, claims.TenantCode, claims.TenantSchema)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// CurrentUserID 便捷读取当前登录账号 ID。
func CurrentUserID(c *gin.Context) uint64 { return ctxkey.UserID(c.Request.Context()) }

// extractToken 按优先级从请求中提取凭证：
// Authorization 头（Bearer 前缀可省略）> WebSocket 子协议 > access_token query。
func extractToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header != "" {
		if after, ok := strings.CutPrefix(header, "Bearer "); ok {
			return strings.TrimSpace(after)
		}
		return strings.TrimSpace(header)
	}
	// 预留：SSE / 文件预览等无法自定义头的场景走 query
	// Browser WebSockets cannot set Authorization. Carry the access token in a
	// separate subprotocol, which is never negotiated back or placed in the URL.
	if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
		for _, protocol := range strings.Split(c.GetHeader("Sec-WebSocket-Protocol"), ",") {
			if token, ok := strings.CutPrefix(strings.TrimSpace(protocol), "bearer."); ok {
				return token
			}
		}
	}
	return c.Query("access_token")
}
