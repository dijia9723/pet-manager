package config

import (
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "config" }

// Register 同时挂载平台侧与租户侧参数配置 CRUD 接口：
// /platform/configs 治理平台系统参数（sys_config），
// /tenant/configs 由租户治理本店参数（t_config，按 schema 隔离）。
func (s *Service) Register(r *router.Router) {
	pg := r.Platform("/configs")
	pg.GET("", r.Require("platform.param.list"), s.handleList)
	pg.POST("", r.Require("platform.param.create"), s.handleCreate)
	pg.PUT("/:id", r.Require("platform.param.update"), s.handleUpdate)
	pg.DELETE("/:id", r.Require("platform.param.delete"), s.handleDelete)

	tg := r.Tenant("/configs")
	tg.GET("", r.Require("tenant.param.list"), s.handleTenantList)
	tg.POST("", r.Require("tenant.param.create"), s.handleTenantCreate)
	tg.PUT("/:id", r.Require("tenant.param.update"), s.handleTenantUpdate)
	tg.DELETE("/:id", r.Require("tenant.param.delete"), s.handleTenantDelete)
}
