package audit

import (
	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleList 审计日志分页查询
//
//	@Summary		审计日志列表
//	@Description	只读查询登录后的写操作审计记录（POST/PUT/PATCH/DELETE），支持租户/操作人/时间范围筛选
//	@Tags			平台-审计日志
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int				false	"页码，默认 1"
//	@Param			page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param			tenant_id	query		int				false	"按租户筛选，0/不传为全部"
//	@Param			username	query		string			false	"按操作人筛选"
//	@Success		200			{object}	response.Body	"data 为 response.Page[Item]"
//	@Router			/platform/audit-logs [get]
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
