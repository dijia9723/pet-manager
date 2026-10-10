package hosting

import "pet-manager/server/internal/router"

// Name 实现 router.Module，返回模块注册名。
func (s *Service) Name() string { return "hosting" }

// Register 挂载托管模块路由：平台侧服务器/网站管理与文件接口，
// 租户侧仅开放归属自身的网站与文件（归属过滤在 service 层完成）。
func (s *Service) Register(r *router.Router) {
	p := r.Platform("/servers")
	p.GET("", r.Require("platform.server.list"), s.handleServers)
	p.POST("", r.Require("platform.server.create"), s.handleSaveServer)
	p.PUT("/:id", r.Require("platform.server.update"), s.handleUpdateServer)
	p.DELETE("/:id", r.Require("platform.server.delete"), s.handleDeleteServer)
	p.POST("/:id/test", r.Require("platform.server.list"), s.handleTestServer)
	p.POST("/:id/exec", r.Require("platform.server.exec"), s.handleExec)
	p.GET("/:id/terminal", r.Require("platform.server.exec"), s.handleTerminal)
	w := r.Platform("/websites")
	w.GET("", r.Require("platform.website.list"), s.handleWebsites)
	w.POST("", r.Require("platform.website.create"), s.handleSaveWebsite)
	w.PUT("/:id", r.Require("platform.website.update"), s.handleUpdateWebsite)
	w.POST("/:id/provision", r.Require("platform.website.provision"), s.handleProvision)
	w.POST("/:id/cms/deploy", r.Require("platform.website.deploy"), s.handleDeployCMS)
	s.registerFiles(w, r, "platform")
	// Platform-owned allocations need ownership filtering, not a long tenant schema transaction.
	t := r.Tenant("/websites", router.TransactionInService)
	t.GET("", r.Require("tenant.website.list"), s.handleWebsites)
	s.registerFiles(t, r, "tenant")
}
