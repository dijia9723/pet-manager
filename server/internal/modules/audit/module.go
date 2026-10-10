package audit

import (
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "platform.audit" }

// Register 挂载平台侧审计日志查询接口（只读）。
func (s *Service) Register(r *router.Router) {
	g := r.Platform("/audit-logs")
	g.GET("", r.Require("platform.audit.list"), s.handleList)
}
