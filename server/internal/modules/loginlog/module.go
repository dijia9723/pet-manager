package loginlog

import (
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "platform.loginlog" }

// Register 挂载平台侧登录日志查询接口（只读，不暴露写接口）。
func (s *Service) Register(r *router.Router) {
	g := r.Platform("/login-logs")
	g.GET("", r.Require("platform.loginlog.list"), s.handleList)
}
