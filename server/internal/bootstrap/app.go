// Package bootstrap 负责把配置、基础设施、中间件与业务模块装配成一个可运行的应用。
//
// 装配顺序刻意保持"依赖在前"的线性关系，任何一个环节失败都直接返回错误，
// 不做降级：多租户系统里"带病启动"比启动失败更危险（可能写错 schema）。
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"

	// 匿名导入触发 swag 生成物注册，缺失会导致 /swagger/doc.json 404
	_ "pet-manager/server/docs"
	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/cache"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/logger"
	"pet-manager/server/internal/infra/objectstore"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/middleware"
	"pet-manager/server/internal/migrate"
	"pet-manager/server/internal/modules/audit"
	"pet-manager/server/internal/modules/auth"
	sysconfig "pet-manager/server/internal/modules/config"
	"pet-manager/server/internal/modules/dict"
	"pet-manager/server/internal/modules/hosting"
	"pet-manager/server/internal/modules/job"
	"pet-manager/server/internal/modules/loginlog"
	"pet-manager/server/internal/modules/notification"
	"pet-manager/server/internal/modules/permission"
	"pet-manager/server/internal/modules/rbac"
	"pet-manager/server/internal/modules/storage"
	"pet-manager/server/internal/modules/tenant"
	"pet-manager/server/internal/modules/tenantprofile"
	"pet-manager/server/internal/modules/tenantuser"
	"pet-manager/server/internal/pkg/jwtx"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/router"
	"pet-manager/server/internal/seed"
)

// 启动期使用的常量。
const (
	// rateLimitPerMinute 全站限流阈值：按客户端 IP、每分钟 600 次固定窗口，防止登录接口被批量撞库
	rateLimitPerMinute = 600
	// startupTimeout 启动阶段（建表、种子、存量租户补表等）的总超时，超时即判定启动失败
	startupTimeout = 30 * time.Second
)

// App 持有应用运行所需的全部长生命周期资源，由 New 装配、Run 启动、Shutdown 释放。
type App struct {
	cfg    *config.Config // 生效的配置快照
	log    *zap.Logger    // 全局日志
	db     *db.Manager    // 数据库连接管理（平台库 + 租户 schema 切换）
	cache  *cache.Client  // Redis 客户端（会话、权限缓存、限流等）
	audit  *audit.Service // 审计服务，退出时用于 flush 缓冲队列
	job    *job.Service   // 定时任务调度器，退出时用于停止调度循环
	engine *gin.Engine    // HTTP 路由引擎
	server *http.Server   // 底层 HTTP 服务
}

// New 完成全部装配。返回后即可直接 Run。
func New(cfg *config.Config) (*App, error) {
	// 1) 基础设施：日志器与数据库连接池（平台库）
	log, err := logger.New(cfg.Log)
	if err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	manager, err := db.New(cfg.Database, log)
	if err != nil {
		return nil, err
	}

	// 2) 启动期数据准备，统一受 startupTimeout 约束
	ctx, cancel := context.WithTimeout(context.Background(), startupTimeout)
	defer cancel()

	// 平台库结构 + 权限点字典 + 平台管理员
	if err := migrate.Platform(ctx, manager); err != nil {
		return nil, err
	}
	if err := seed.Run(ctx, manager, cfg.Seed, log); err != nil {
		return nil, err
	}
	// 存量租户批量补表：W2 之后新增的宠物档案/工单表，靠这一步推给老租户
	if err := migrate.AllTenants(ctx, manager, log); err != nil {
		return nil, err
	}

	// 3) 缓存与会话：Redis 客户端承载会话、权限缓存与限流计数
	cacheClient, err := cache.New(cfg.Redis)
	if err != nil {
		return nil, err
	}
	sessions := session.New(cacheClient)
	// 启动时种子可能补充了权限点，清理旧权限缓存使新功能立即按最新字典鉴权。
	if err := sessions.InvalidateAllPermissions(ctx); err != nil {
		return nil, fmt.Errorf("刷新权限缓存失败: %w", err)
	}
	// JWT 签发与校验管理器，供登录与鉴权中间件共用
	jwtMgr := jwtx.New(cfg.JWT)

	// 4) 业务服务装配（顺序即依赖顺序）
	// 业务服务：rbac 提供权限解析，tenant 提供租户快照，都是中间件的依赖
	rbacSvc := rbac.NewService(manager, sessions, log)
	tenantSvc := tenant.NewService(manager, sessions, cacheClient, rbacSvc, log)
	// 登录日志模块需先于 auth 构造，供其采集登录事件
	loginlogSvc := loginlog.NewService(manager, log)
	authSvc := auth.NewService(manager, sessions, jwtMgr, rbacSvc, loginlogSvc, log)
	userSvc := tenantuser.NewService(sessions, log)
	profileSvc := tenantprofile.NewService(manager, sessions, log)
	auditSvc := audit.NewService(manager, log)
	permSvc := permission.NewService(manager, sessions, log)
	configSvc := sysconfig.NewService(manager)
	dictSvc := dict.NewService(manager)
	// 托管服务：负责 SSH 密钥加密等，配置缺失会直接失败
	hostingSvc, err := hosting.NewService(manager, cfg.Hosting, cfg.CORS.AllowOrigins...)
	if err != nil {
		return nil, err
	}
	// 回收上次进程异常退出时中断的托管任务
	if err := hostingSvc.RecoverInterrupted(ctx); err != nil {
		return nil, err
	}
	// 对象存储：TOS 未启用时 objectStore 为 nil，storage 服务退化为占位实现
	objectStore, err := objectstore.New(cfg.TOS)
	if err != nil {
		return nil, err
	}
	storageSvc := storage.NewService(manager, nil, cfg.TOS)
	if objectStore != nil {
		storageSvc = storage.NewService(manager, objectStore, cfg.TOS)
	}
	notificationSvc := notification.NewService(manager, storageSvc)

	// 定时任务模块：先建调度器，再由各业务模块注册内置任务执行体
	jobSvc := job.NewService(manager, cacheClient, log, job.ExecutorOptions{ShellPath: cfg.Jobs.ShellPath})
	jobSvc.RegisterBuiltin(job.Builtin{Code: "storage_cleanup", Name: "存储资源清理", Spec: "*/5 * * * *", Remark: "重试待清理的 TOS 对象，回收半小时未完成上传并释放预占配额", Fn: storageSvc.Cleanup})
	jobSvc.RegisterBuiltin(job.Builtin{
		Code:   "tenant_purge",
		Name:   "租户数据销毁",
		Spec:   "30 3 * * *", // 每日 03:30，低峰期回收到期的待销毁租户
		Remark: "销毁已过宽限期（默认 7 天，参数 tenant.purge_after_days 可配）的待销毁租户：DROP SCHEMA 并清理平台侧账号与租户记录，审计/登录日志保留",
		Fn:     tenantSvc.RunPurge,
	})

	if cfg.App.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}
	// 5) HTTP 引擎与全局中间件：按注册顺序组成拦截器链，顺序即执行顺序
	engine := gin.New()

	// recovery 最先（兜底 panic）-> request_id -> 安全头 -> CORS -> 限流 -> 访问日志 -> 审计
	registry := middleware.NewRegistry().
		Add("recovery", middleware.OrderRecovery, middleware.Recovery(log)).
		Add("request_id", middleware.OrderRequestID, middleware.RequestID()).
		Add("security_headers", middleware.OrderSecurityHeaders, middleware.SecurityHeaders()).
		Add("cors", middleware.OrderCORS, middleware.CORS(cfg.CORS)).
		Add("rate_limit", middleware.OrderRateLimit, middleware.RateLimit(cacheClient, rateLimitPerMinute, time.Minute)).
		Add("access_log", middleware.OrderAccessLog, middleware.AccessLog(log)).
		Add("audit", middleware.OrderAudit, middleware.Audit(auditSvc))
	engine.Use(registry.Handlers()...)

	// 审计落库 worker 与 HTTP 服务同生命周期
	auditSvc.Run(context.Background())
	// 定时任务调度器与 HTTP 服务同生命周期
	jobSvc.Run(context.Background())

	// 6) 路由与鉴权依赖：Auth 校验 JWT，Guard 校验租户状态，RBAC 校验权限点，Scope 切换 schema
	r := router.New(engine, router.Deps{
		Auth:  middleware.NewAuthenticator(jwtMgr, sessions),
		Guard: middleware.NewTenantGuard(tenantSvc),
		RBAC:  middleware.NewRBAC(rbacSvc, sessions),
		Scope: middleware.TenantScope(manager),
	})

	// 探活放在鉴权链路之外，供容器/K8s 直接调用
	engine.GET("/healthz", func(c *gin.Context) {
		sqlDB, err := manager.Platform().DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			response.Fail(c, errors.New("数据库不可用"))
			return
		}
		response.OK(c, gin.H{"status": "ok", "env": cfg.App.Env})
	})

	// Swagger 仅在非生产环境开放，避免暴露接口清单；访问 /swagger/index.html
	if !cfg.App.IsProd() {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(
			swaggerFiles.Handler,
			ginSwagger.URL("/swagger/doc.json"),
		))
		log.Info("Swagger 文档已启用", zap.String("url", "http://"+cfg.App.Addr+"/swagger/index.html"))
	}

	// 7) 挂载所有业务模块：每个模块通过 router.Module 接口自行注册路由
	modules := []router.Module{authSvc, tenantSvc, rbacSvc, userSvc, profileSvc, auditSvc, permSvc, configSvc, dictSvc, loginlogSvc, jobSvc, notificationSvc, storageSvc, hostingSvc}
	moduleNames := r.Mount(modules...)

	log.Info("HTTP 拦截器链已装配", zap.Strings("middlewares", registry.Names()))
	log.Info("业务模块已挂载", zap.Strings("modules", moduleNames))

	// 装配完成，返回可运行的 App（此时尚未监听端口）
	return &App{
		cfg:    cfg,
		log:    log,
		db:     manager,
		cache:  cacheClient,
		audit:  auditSvc,
		job:    jobSvc,
		engine: engine,
		server: &http.Server{
			Addr:              cfg.App.Addr,
			Handler:           engine,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}, nil
}

// Engine 暴露底层引擎，供测试注入 httptest 使用。
func (a *App) Engine() *gin.Engine { return a.engine }

// Run 启动 HTTP 服务，阻塞至服务退出。
func (a *App) Run() error {
	a.log.Info("HTTP 服务已启动", zap.String("addr", a.cfg.App.Addr), zap.String("env", a.cfg.App.Env))
	// ErrServerClosed 是 Shutdown 触发的正常关闭，不当成错误上报
	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown 优雅退出：先停 HTTP 再放连接池与 Redis，保证在途请求写库完整。
func (a *App) Shutdown(ctx context.Context) error {
	// 先停止接收新请求并等待在途请求结束（受 ctx 超时约束）
	if err := a.server.Shutdown(ctx); err != nil {
		a.log.Error("HTTP 服务关闭异常", zap.Error(err))
	}
	// HTTP 停完后 flush 审计队列，避免丢失退关前的写操作记录
	a.audit.Close()
	// 停止定时任务调度循环
	a.job.Close()
	// 最后释放数据库与 Redis 连接
	if err := a.db.Close(); err != nil {
		a.log.Error("关闭数据库连接失败", zap.Error(err))
	}
	if err := a.cache.Close(); err != nil {
		a.log.Error("关闭 Redis 连接失败", zap.Error(err))
	}
	a.log.Info("服务已退出")
	return nil
}
