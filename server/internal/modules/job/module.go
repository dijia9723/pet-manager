package job

import (
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "platform.job" }

// Register 挂载平台侧任务管理接口：方法目录、创建/删除、启停/编辑、执行和日志。
func (s *Service) Register(r *router.Router) {
	g := r.Platform("/jobs")
	g.GET("", r.Require("platform.job.list"), s.handleList)
	g.GET("/methods", r.Require("platform.job.list"), s.handleMethods)
	g.POST("", r.Require("platform.job.create"), s.handleCreate)
	g.DELETE("/:id", r.Require("platform.job.delete"), s.handleDelete)
	g.PUT("/:id", r.Require("platform.job.update"), s.handleUpdate)
	g.POST("/:id/enable", r.Require("platform.job.update"), s.handleEnable)
	g.POST("/:id/disable", r.Require("platform.job.update"), s.handleDisable)
	g.POST("/:id/run", r.Require("platform.job.trigger"), s.handleRun)
	g.GET("/:id/logs", r.Require("platform.job.log"), s.handleLogs)
}
