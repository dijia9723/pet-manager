package permission

import (
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "platform.permission" }

// Register 挂载平台侧权限点（菜单）配置接口。
func (s *Service) Register(r *router.Router) {
	g := r.Platform("/permissions")
	g.GET("/tree", r.Require("platform.permission.list"), s.handleTree)
	g.POST("", r.Require("platform.permission.create"), s.handleCreate)
	g.PUT("/:id", r.Require("platform.permission.update"), s.handleUpdate)
	g.DELETE("/:id", r.Require("platform.permission.delete"), s.handleDelete)
}
