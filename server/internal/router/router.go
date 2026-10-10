// Package router 负责路由装配与模块挂载。
//
// 模块只需要声明"我要挂在哪个命名空间、需要什么权限点"，
// 鉴权链路（凭证解析 -> 租户门禁 -> 租户事务 -> 权限点）由这里统一施加，
// 业务模块不再关心中间件顺序。
package router

import (
	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/middleware"
	"pet-manager/server/internal/pkg/ctxkey"
)

// Deps 是路由装配需要的鉴权组件。
// Scope 单独以 HandlerFunc 形式注入，避免 router 反向依赖 db 包。
type Deps struct {
	Auth  *middleware.Authenticator // 凭证解析
	Guard *middleware.TenantGuard   // 租户状态门禁
	RBAC  *middleware.RBAC          // 权限点校验
	Scope gin.HandlerFunc           // 租户请求级事务（middleware.TenantScope）
}

// Router 包装 gin.Engine，提供按身份划分的路由分组。
type Router struct {
	engine *gin.Engine // 底层引擎
	deps   Deps        // 鉴权组件
}

// New 构造路由装配器。
func New(engine *gin.Engine, deps Deps) *Router {
	return &Router{engine: engine, deps: deps}
}

// Engine 暴露底层引擎，供第三方中间件或静态资源挂载使用。
func (r *Router) Engine() *gin.Engine { return r.engine }

// V1 返回 /api/v1 版本根分组。
func (r *Router) V1() *gin.RouterGroup { return r.engine.Group("/api/v1") }

// Public 无需登录的接口（登录、健康检查、回调等）。
func (r *Router) Public(path string) *gin.RouterGroup {
	return r.V1().Group(path)
}

// Authed 需要登录但不区分平台/租户身份的接口（个人信息、登出）。
func (r *Router) Authed(path string) *gin.RouterGroup {
	g := r.V1().Group(path)
	g.Use(r.deps.Auth.Require(""))
	return g
}

// Platform 平台端接口：平台管理员身份。
func (r *Router) Platform(path string) *gin.RouterGroup {
	g := r.V1().Group("/platform" + path)
	g.Use(r.deps.Auth.Require(ctxkey.UserTypePlatform))
	return g
}

// TenantScopeMode 控制事务生命周期，默认请求级事务。
type TenantScopeMode uint8

// TransactionInService 仅用于跨网络对象操作，服务必须自行通过 Manager.TenantTx 管理短事务。
// 保留身份与租户状态门禁，不开启空外层事务，避免并发请求占满连接池后等待第二个连接。
const TransactionInService TenantScopeMode = 1

// Tenant 租户端接口：租户身份 + 租户状态门禁 + 租户事务。
// 默认通过 Scope 隔离；显式 TransactionInService 时由服务使用 TenantTx 完成隔离。
func (r *Router) Tenant(path string, mode ...TenantScopeMode) *gin.RouterGroup {
	g := r.V1().Group("/tenant" + path)
	g.Use(
		r.deps.Auth.Require(ctxkey.UserTypeTenant),
		r.deps.Guard.Context(),
	)
	if len(mode) == 0 || mode[0] != TransactionInService {
		g.Use(r.deps.Scope)
	}
	return g
}

// Require 声明接口所需权限点，供模块在注册路由时使用。
func (r *Router) Require(codes ...string) gin.HandlerFunc {
	return r.deps.RBAC.Require(codes...)
}

// Module 是一个可挂载的业务模块。
// 模块自行决定路由挂在 Public / Authed / Platform / Tenant 哪一类分组下。
type Module interface {
	Name() string       // 模块名，用于启动日志与排障
	Register(r *Router) // 注册路由
}

// Mount 按注册顺序挂载全部模块，并返回模块名列表用于启动日志。
func (r *Router) Mount(modules ...Module) []string {
	names := make([]string, 0, len(modules))
	for _, m := range modules {
		m.Register(r)
		names = append(names, m.Name())
	}
	return names
}
