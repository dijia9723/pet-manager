package loginlog

import (
	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleList 登录日志分页查询
//
//	@Summary		登录日志列表
//	@Description	只读查询全部登录尝试（含失败），支持用户名/租户/结果/时间范围筛选
//	@Tags			平台-登录日志
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int				false	"页码，默认 1"
//	@Param			page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param			username	query		string			false	"按登录名筛选"
//	@Param			status		query		string			false	"结果筛选 success/fail"
//	@Success		200			{object}	response.Body	"data 为 response.Page[Item]"
//	@Router			/platform/login-logs [get]
func (s *Service) handleList(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	page, err := s.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, page)
}
