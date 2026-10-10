package notification

import "pet-manager/server/internal/router"

// Name 返回模块名，用于服务注册与日志标识。
func (s *Service) Name() string { return "notification" }

// Register 挂载通知模块路由：平台/租户分挂通知管理与消息中心；
// 消息中心为账号级接口，不加角色权限，统一按接收快照鉴权。
func (s *Service) Register(r *router.Router) {
	p := r.Platform("/notifications")
	p.GET("", r.Require("platform.notification.list"), s.handleList)
	p.GET("/targets", r.Require("platform.notification.create"), s.handleTargets)
	p.GET("/:id/attachments/:resource_id/download", r.Require("platform.notification.list"), s.handleManagedAttachment)
	p.POST("", r.Require("platform.notification.create"), s.handleCreate)
	p.PUT("/:id", r.Require("platform.notification.create"), s.handleUpdate)
	p.POST("/:id/publish", r.Require("platform.notification.publish"), s.handlePublish)
	p.POST("/:id/revoke", r.Require("platform.notification.revoke"), s.handleRevoke)
	p.GET("/:id/receipts", r.Require("platform.notification.list"), s.handleReceipts)

	t := r.Tenant("/notifications")
	t.GET("/:id/attachments/:resource_id/download", r.Require("tenant.notification.list"), s.handleManagedAttachment)
	t.GET("", r.Require("tenant.notification.list"), s.handleList)
	t.GET("/targets", r.Require("tenant.notification.create"), s.handleTargets)
	t.POST("", r.Require("tenant.notification.create"), s.handleCreate)
	t.PUT("/:id", r.Require("tenant.notification.create"), s.handleUpdate)
	t.POST("/:id/publish", r.Require("tenant.notification.publish"), s.handlePublish)
	t.POST("/:id/revoke", r.Require("tenant.notification.revoke"), s.handleRevoke)
	t.GET("/:id/receipts", r.Require("tenant.notification.list"), s.handleReceipts)

	// 个人收件箱无需额外角色权限；租户端保留状态门禁和请求事务。
	pg := r.Platform("/messages")
	pg.GET("/:id/attachments/:resource_id/download", s.handleReceivedAttachment)
	pg.GET("", s.handleInbox)
	pg.GET("/unread-count", s.handleUnread)
	pg.POST("/:id/read", s.handleRead)
	pg.POST("/:id/process", s.handleProcess)
	tg := r.Tenant("/messages")
	tg.GET("/:id/attachments/:resource_id/download", s.handleReceivedAttachment)
	tg.GET("", s.handleInbox)
	tg.GET("/unread-count", s.handleUnread)
	tg.POST("/:id/read", s.handleRead)
	tg.POST("/:id/process", s.handleProcess)
}
