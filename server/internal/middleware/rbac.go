package middleware

import (
	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// PermissionWildcard 是平台管理员的通配权限点。
const PermissionWildcard = "*"

// RBAC 负责权限点校验。
//
// 权限点集合按"租户 + 账号"缓存，角色或授权变更时由业务侧失效缓存，
// 因此这里正常路径上只有一次 Redis 读取。
type RBAC struct {
	resolver PermissionResolver
	sessions *session.Store
}

// NewRBAC 构造权限校验器。
func NewRBAC(resolver PermissionResolver, sessions *session.Store) *RBAC {
	return &RBAC{resolver: resolver, sessions: sessions}
}

// Require 声明某条路由需要的权限点（满足任一即可）。
// 用法：r.Tenant("role.list", mw.Require("tenant.role.list")).
func (r *RBAC) Require(codes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms, err := r.load(c)
		if err != nil {
			response.Abort(c, err)
			return
		}
		if len(codes) == 0 {
			c.Next()
			return
		}
		if !HasAny(perms, codes) {
			response.Abort(c, errcode.New(errcode.ErrForbiddenAction))
			return
		}
		c.Next()
	}
}

// load 解析当前账号权限点并写入上下文（幂等，同一请求只算一次）。
func (r *RBAC) load(c *gin.Context) ([]string, error) {
	ctx := c.Request.Context()
	if cached := ctxkey.Permissions(ctx); cached != nil {
		return cached, nil
	}

	var perms []string
	switch ctxkey.UserType(ctx) {
	case ctxkey.UserTypePlatform:
		// 平台管理员按通配处理：权限点字典是平台自己维护的，无需再自我授权
		perms = []string{PermissionWildcard}
	default:
		tenantID := ctxkey.TenantID(ctx)
		userID := ctxkey.UserID(ctx)
		schema := ctxkey.TenantSchema(ctx)
		if tenantID == 0 || userID == 0 {
			return nil, errcode.New(errcode.ErrUnauthorized)
		}

		// 先查 Redis 缓存，未命中再回源解析并回填；缓存写入失败不影响本次放行
		if cached, ok := r.sessions.CachedPermissions(ctx, tenantID, userID); ok {
			perms = cached
		} else {
			resolved, err := r.resolver.ResolvePermissions(ctx, tenantID, userID, schema)
			if err != nil {
				return nil, err
			}
			perms = resolved
			_ = r.sessions.CachePermissions(ctx, tenantID, userID, perms)
		}
	}

	c.Request = c.Request.WithContext(ctxkey.WithPermissions(ctx, perms))
	return perms, nil
}

// HasAny 判断权限集合是否命中所需权限之一（支持通配）。
func HasAny(owned []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, o := range owned {
		if o == PermissionWildcard {
			return true
		}
	}
	for _, need := range required {
		for _, o := range owned {
			if o == need {
				return true
			}
		}
	}
	return false
}
