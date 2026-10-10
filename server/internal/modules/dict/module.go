package dict

import (
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "dict" }

// Register 同时挂载平台侧与租户侧字典管理接口（字典类型 + 字典项）：
// /platform/dicts 治理平台字典（sys_dict*），
// /tenant/dicts 由租户治理本店字典（t_dict*，按 schema 隔离）。
func (s *Service) Register(r *router.Router) {
	pg := r.Platform("/dicts")
	pg.GET("", r.Require("platform.dict.list"), s.handleListDicts)
	pg.POST("", r.Require("platform.dict.create"), s.handleCreateDict)
	pg.PUT("/:id", r.Require("platform.dict.update"), s.handleUpdateDict)
	pg.DELETE("/:id", r.Require("platform.dict.delete"), s.handleDeleteDict)

	pg.GET("/:id/items", r.Require("platform.dict.list"), s.handleListItems)
	pg.POST("/:id/items", r.Require("platform.dict.create"), s.handleCreateItem)
	pg.PUT("/:id/items/:itemId", r.Require("platform.dict.update"), s.handleUpdateItem)
	pg.DELETE("/:id/items/:itemId", r.Require("platform.dict.delete"), s.handleDeleteItem)

	tg := r.Tenant("/dicts")
	tg.GET("", r.Require("tenant.dict.list"), s.handleTenantListDicts)
	tg.POST("", r.Require("tenant.dict.create"), s.handleTenantCreateDict)
	tg.PUT("/:id", r.Require("tenant.dict.update"), s.handleTenantUpdateDict)
	tg.DELETE("/:id", r.Require("tenant.dict.delete"), s.handleTenantDeleteDict)

	tg.GET("/:id/items", r.Require("tenant.dict.list"), s.handleTenantListItems)
	tg.POST("/:id/items", r.Require("tenant.dict.create"), s.handleTenantCreateItem)
	tg.PUT("/:id/items/:itemId", r.Require("tenant.dict.update"), s.handleTenantUpdateItem)
	tg.DELETE("/:id/items/:itemId", r.Require("tenant.dict.delete"), s.handleTenantDeleteItem)
}
