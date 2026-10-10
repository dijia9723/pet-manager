// Package ctxkey 集中管理 context.Context 中的键，避免各模块散落魔法字符串。
package ctxkey

import (
	"context"

	"gorm.io/gorm"

	"pet-manager/server/internal/pkg/errcode"
)

// key 是自定义的 context 键类型。用自定义类型而非裸 string 作键，
// 可避免与第三方库的键发生碰撞。
type key string

const (
	// 以下键均为包内私有，跨模块只能通过配套的 With/Getter 读写，
	// 以保证类型安全并收敛键名。
	keyRequestID    key = "request_id"    // 请求追踪 ID
	keyUserID       key = "user_id"       // 当前登录账号 ID
	keyUsername     key = "username"      // 当前登录账号名
	keyUserType     key = "user_type"     // 账号类型：platform / tenant
	keySessionID    key = "session_id"    // 会话 ID，用于强制下线
	keyTenantID     key = "tenant_id"     // 租户 ID
	keyTenantCode   key = "tenant_code"   // 租户编码
	keyTenantSchema key = "tenant_schema" // 租户业务数据所在 schema
	keyPermissions  key = "permissions"   // 当前账号权限点集合
	keyTenantDB     key = "tenant_db"     // 请求级租户事务句柄
	keyTraceID      key = "trace_id"      // 链路追踪 ID（预留）
)

// WithRequestID 注入请求追踪 ID，供日志与审计串联同一请求。
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

// RequestID 读取请求追踪 ID，缺失时返回空串。
func RequestID(ctx context.Context) string { return stringValue(ctx, keyRequestID) }

// WithUser 一次性写入当前登录账号的身份三要素。
// access/refresh 解析成功后由鉴权中间件调用。
func WithUser(ctx context.Context, userID uint64, username, userType string) context.Context {
	ctx = context.WithValue(ctx, keyUserID, userID)
	ctx = context.WithValue(ctx, keyUsername, username)
	ctx = context.WithValue(ctx, keyUserType, userType)
	return ctx
}

// WithSession 记录当前请求所用凭证的会话 ID，登出时据此注销会话。
func WithSession(ctx context.Context, sid string) context.Context {
	return context.WithValue(ctx, keySessionID, sid)
}

// SessionID 读取当前请求所用凭证的会话 ID。
func SessionID(ctx context.Context) string { return stringValue(ctx, keySessionID) }

// UserID 读取当前登录账号 ID，未登录返回 0。
func UserID(ctx context.Context) uint64 {
	if v, ok := ctx.Value(keyUserID).(uint64); ok {
		return v
	}
	return 0
}

// Username 读取当前登录账号名，缺失时返回空串。
func Username(ctx context.Context) string { return stringValue(ctx, keyUsername) }

// UserType 取值为 UserTypePlatform / UserTypeTenant。
func UserType(ctx context.Context) string { return stringValue(ctx, keyUserType) }

// 账号类型常量，与 jwtx.Claims.UserType 及路由分组一一对应。
const (
	UserTypePlatform = "platform" // 平台管理员
	UserTypeTenant   = "tenant"   // 租户侧账号
)

// WithTenant 写入租户上下文三元组。仅租户身份账号会调用。
func WithTenant(ctx context.Context, tenantID uint64, code, schema string) context.Context {
	ctx = context.WithValue(ctx, keyTenantID, tenantID)
	ctx = context.WithValue(ctx, keyTenantCode, code)
	ctx = context.WithValue(ctx, keyTenantSchema, schema)
	return ctx
}

// TenantID 读取租户 ID，非租户上下文返回 0。
func TenantID(ctx context.Context) uint64 {
	if v, ok := ctx.Value(keyTenantID).(uint64); ok {
		return v
	}
	return 0
}

// TenantCode 读取租户编码，缺失时返回空串。
func TenantCode(ctx context.Context) string { return stringValue(ctx, keyTenantCode) }

// TenantSchema 是租户业务数据所在 schema，形如 tenant_12。
// 该值由系统生成并经过正则校验，不接受用户输入，因此可直接用于 search_path。
func TenantSchema(ctx context.Context) string { return stringValue(ctx, keyTenantSchema) }

// WithPermissions 缓存当前请求解析出的权限点集合，避免同请求内重复解析。
func WithPermissions(ctx context.Context, perms []string) context.Context {
	return context.WithValue(ctx, keyPermissions, perms)
}

// Permissions 读取权限点缓存；返回 nil 表示尚未解析过（注意：与"空权限集"语义不同）。
func Permissions(ctx context.Context) []string {
	if v, ok := ctx.Value(keyPermissions).([]string); ok {
		return v
	}
	return nil
}

// WithTenantDB 注入请求级的租户事务句柄。
// 租户域的所有仓储只允许使用它拿到的 *gorm.DB，禁止使用全局连接。
func WithTenantDB(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, keyTenantDB, tx)
}

// TenantDB 读取请求级租户事务句柄，未注入时返回 nil。
func TenantDB(ctx context.Context) *gorm.DB {
	if v, ok := ctx.Value(keyTenantDB).(*gorm.DB); ok {
		return v
	}
	return nil
}

// MustTenantDB 是租户域仓储的取数入口。
// 拿不到事务句柄说明该请求没有经过 TenantScope 中间件，属于编码错误，
// 这里直接报错而不是回退到平台连接，避免数据写错 schema。
func MustTenantDB(ctx context.Context) (*gorm.DB, error) {
	tx := TenantDB(ctx)
	if tx == nil {
		return nil, errcode.New(errcode.ErrInternal)
	}
	return tx, nil
}

// stringValue 是各字符串键 Getter 的公共实现：类型不符或键不存在一律返回空串。
func stringValue(ctx context.Context, k key) string {
	if v, ok := ctx.Value(k).(string); ok {
		return v
	}
	return ""
}
