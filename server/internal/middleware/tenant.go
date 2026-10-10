package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// TenantGuard 校验租户可用性：存在、未停用、未到期。
// 数据来源优先走 Redis 快照，避免每请求打平台库。
type TenantGuard struct {
	resolver TenantResolver // 带缓存的租户快照查询
}

// NewTenantGuard 构造租户门禁。
func NewTenantGuard(resolver TenantResolver) *TenantGuard {
	return &TenantGuard{resolver: resolver}
}

// Context 要求当前请求携带租户上下文，并完成状态门禁。
func (g *TenantGuard) Context() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tenantID := ctxkey.TenantID(ctx)
		if tenantID == 0 {
			response.Abort(c, errcode.New(errcode.ErrTenantNotFound))
			return
		}

		info, err := g.resolver.TenantByID(ctx, tenantID)
		if err != nil {
			response.Abort(c, err)
			return
		}
		if info.Status != model.StatusEnabled {
			response.Abort(c, errcode.New(errcode.ErrTenantDisabled))
			return
		}
		if info.IsExpired(time.Now()) {
			response.Abort(c, errcode.New(errcode.ErrTenantExpired))
			return
		}
		// 以数据库为准回填 schema，防止凭证里的陈旧值被使用
		if err := db.ValidateSchema(info.Schema); err != nil {
			response.Abort(c, err)
			return
		}

		c.Request = c.Request.WithContext(ctxkey.WithTenant(ctx, info.ID, info.Code, info.Schema))
		c.Next()
	}
}

// TenantScope 为租户请求开启请求级事务，并在事务内切换到租户 search_path。
//
// 这是"租户业务数据禁止裸连接访问"的执行点：
// 业务代码只能从 ctxkey.TenantDB(ctx) 取事务句柄，事务提交/回滚后连接自动复位，
// 连接池复用时不会残留其他租户的 search_path。
//
// 注意：本中间件会把事务持有到请求结束，因此租户域接口不适合做超长耗时操作
// （大文件上传、SSE 长连接请走非租户域路由）。
func TenantScope(m *db.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		schema := ctxkey.TenantSchema(ctx)
		if schema == "" {
			response.Abort(c, errcode.New(errcode.ErrTenantNotFound))
			return
		}

		// 开启事务并在事务内切换 search_path；句柄注入上下文供仓储层取用
		tx, err := m.BeginTenant(ctx, schema)
		if err != nil {
			response.Abort(c, err)
			return
		}

		// panic 时确保回滚再抛，避免事务泄漏占用连接
		defer func() {
			if r := recover(); r != nil {
				_ = m.EndTenant(tx, false)
				panic(r)
			}
		}()

		c.Request = c.Request.WithContext(ctxkey.WithTenantDB(ctx, tx))
		c.Next()

		// 响应状态为 4xx/5xx 视为业务失败，整体回滚；否则提交
		if err := m.EndTenant(tx, c.Writer.Status() < 400); err != nil {
			// 事务已经无法挽救，只能记录并覆盖响应
			response.Abort(c, err)
		}
	}
}
