package storage

import "pet-manager/server/internal/router"

// Name 返回模块名，用于服务注册与日志标识。
func (s *Service) Name() string { return "storage" }

// Register 挂载存储模块路由：平台端与租户端分别校验各自权限点；
// 涉及跨 TOS 的写操作使用 TransactionInService，避免长耗时上传期间占用请求连接或外层事务。
func (s *Service) Register(r *router.Router) {
	p := r.Platform("/storage")
	p.GET("/resources", r.Require("platform.storage.list", "platform.notification.create"), s.handleList)
	p.GET("/summary", r.Require("platform.storage.list", "platform.notification.create"), s.handleSummary)
	p.POST("/resources", r.Require("platform.storage.upload", "platform.notification.create"), s.handleUpload)
	p.DELETE("/resources/:id", r.Require("platform.storage.delete"), s.handleDelete)
	p.GET("/resources/:id/download", r.Require("platform.storage.list"), s.handleDownload)
	p.GET("/plans", r.Require("platform.storage.list"), s.handlePlans)
	p.GET("/accounts", r.Require("platform.storage.list"), s.handleAccounts)
	p.PUT("/accounts/:id", r.Require("platform.storage.allocate"), s.handleAllocate)
	p.GET("/accounts/:id/changes", r.Require("platform.storage.list"), s.handleChanges)
	t := r.Tenant("/storage")
	t.GET("/resources", r.Require("tenant.storage.list", "tenant.notification.create"), s.handleList)
	t.GET("/resources/:id/download", r.Require("tenant.storage.list"), s.handleDownload)
	// 跨 TOS 操作仅持有服务内短 TenantTx，不占用空外层连接或事务。
	w := r.Tenant("/storage", router.TransactionInService)
	w.GET("/summary", r.Require("tenant.storage.list", "tenant.notification.create"), s.handleSummary)
	w.POST("/resources", r.Require("tenant.storage.upload", "tenant.notification.create"), s.handleUpload)
	w.DELETE("/resources/:id", r.Require("tenant.storage.delete"), s.handleDelete)
}
