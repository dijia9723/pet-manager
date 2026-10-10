package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/ctxkey"
)

// AuditEntry 是一次写操作的审计记录。
//
// 刻意不含请求体/响应体：重置密码等接口的明文口令绝不能进审计表，
// 追溯依赖 route + status + 操作人 + request_id 回查访问日志。
type AuditEntry struct {
	TenantID  uint64 // 租户 ID，平台操作时为 0
	UserType  string // platform / tenant
	UserID    uint64 // 操作人账号 ID
	Username  string // 操作人账号名
	Method    string // HTTP 方法
	Path      string // 实际请求路径（可能含具体 id）
	Route     string // 路由模板（稳定），用于聚合统计
	Status    int    // 响应 HTTP 状态码
	ClientIP  string
	UserAgent string
	LatencyMs int64  // 请求耗时（毫秒）
	RequestID string // 关联访问日志
}

// AuditRecorder 由审计模块实现，中间件只依赖接口，避免反向依赖 modules。
// 沿用 PermissionResolver / TenantResolver 的既有先例。
type AuditRecorder interface {
	Submit(AuditEntry)
	// ShouldRecord 判定请求是否纳入审计：总开关 / 方法白名单 / 排除路由
	// 均由 sys_config 的 audit 分组参数治理（约 60s 生效），中间件不内置规则。
	ShouldRecord(method, route string) bool
}

// Audit 记录写操作审计日志。
//
// 注册在收尾序位（OrderAudit=900，全局）：c.Next() 之前先放行，
// 等 Auth / TenantScope 等组中间件执行完、响应状态确定后再读上下文落库，
// 因此这里拿到的是最终 user_type / tenant_id / status。
// 未登录（UserType 为空）直接跳过，登录、刷新、公开路由天然不记。
// 是否记录交给 recorder.ShouldRecord（默认策略：登录后的 POST/PUT/PATCH/DELETE）。
// Submit 为非阻塞投递，审计失败绝不影响主请求。
func Audit(rec AuditRecorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rec == nil {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		ctx := c.Request.Context()
		userType := ctxkey.UserType(ctx)
		if userType == "" {
			return
		}

		// 优先用路由模板（稳定、不含具体 id），404 等未命中场景回退实际路径。
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		if !rec.ShouldRecord(c.Request.Method, route) {
			return
		}

		rec.Submit(AuditEntry{
			TenantID:  ctxkey.TenantID(ctx),
			UserType:  userType,
			UserID:    ctxkey.UserID(ctx),
			Username:  ctxkey.Username(ctx),
			Method:    c.Request.Method,
			Path:      c.Request.URL.Path,
			Route:     c.FullPath(),
			Status:    c.Writer.Status(),
			ClientIP:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			LatencyMs: time.Since(start).Milliseconds(),
			RequestID: ctxkey.RequestID(ctx),
		})
	}
}
