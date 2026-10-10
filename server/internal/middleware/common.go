package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/cache"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/pkg/security"
)

// RequestID 为每个请求分配唯一 ID，贯穿日志、响应头，方便排障与前端反馈定位。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" {
			id = security.RandomToken(12)
		}
		c.Set("request_id", id)
		c.Request = c.Request.WithContext(ctxkey.WithRequestID(c.Request.Context(), id))
		c.Header("X-Request-Id", id)
		c.Next()
	}
}

// Recovery 兜底 panic，保证单个请求异常不会打挂进程。
func Recovery(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("请求处理发生 panic",
					zap.String("request_id", ctxkey.RequestID(c.Request.Context())),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.Any("panic", r),
					zap.Stack("stack"),
				)
				response.Abort(c, errcode.Wrap(errcode.ErrInternal, fmt.Errorf("panic: %v", r)))
			}
		}()
		c.Next()
	}
}

// SecurityHeaders 补几个基础安全响应头，避免每个服务各写一遍。
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// CORS 按白名单放行，未命中来源直接拒绝。
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowOrigins))
	for _, o := range cfg.AllowOrigins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed[origin]; ok && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Authorization,X-Request-Id,X-Client")
			c.Header("Access-Control-Expose-Headers", "X-Request-Id")
			c.Header("Access-Control-Max-Age", "86400")
		}
		// 预检请求不进入业务 handler，直接以 204 结束
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// AccessLog 输出结构化访问日志，租户与账号维度都带上，便于按租户排查问题。
func AccessLog(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()

		ctx := c.Request.Context()
		fields := []zap.Field{
			zap.String("request_id", ctxkey.RequestID(ctx)),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Int("bytes", c.Writer.Size()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
			zap.String("user_type", ctxkey.UserType(ctx)),
		}
		if uid := ctxkey.UserID(ctx); uid > 0 {
			fields = append(fields, zap.Uint64("user_id", uid))
		}
		if tid := ctxkey.TenantID(ctx); tid > 0 {
			fields = append(fields, zap.Uint64("tenant_id", tid))
		}
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.String()))
		}

		switch {
		case c.Writer.Status() >= 500:
			log.Error("access", fields...)
		case c.Writer.Status() >= 400:
			log.Warn("access", fields...)
		default:
			log.Info("access", fields...)
		}
	}
}

// RateLimit 基于 Redis 的固定窗口限流。限流组件自身故障时不阻断业务（fail-open）。
func RateLimit(rdb *cache.Client, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rdb == nil || limit <= 0 {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		// 把时间切成固定窗口：同一窗口内共用同一个计数 key，跨窗口自动新计数
		bucket := strconv.FormatInt(time.Now().Unix()/int64(window.Seconds()), 10)
		key := rdb.Key("ratelimit", c.ClientIP(), bucket)

		// 首次自增（返回 1）时设置过期，避免计数 key 永驻；Redis 故障时放行
		n, err := rdb.Raw().Incr(ctx, key).Result()
		if err != nil {
			c.Next()
			return
		}
		if n == 1 {
			_ = rdb.Raw().Expire(ctx, key, window).Err()
		}
		if n > int64(limit) {
			response.Abort(c, errcode.New(errcode.ErrTooManyRequests))
			return
		}
		c.Next()
	}
}
