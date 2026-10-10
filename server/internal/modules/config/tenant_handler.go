package config

import (
	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleTenantList 租户参数配置分页列表
//
//	@Summary	租户参数列表
//	@Tags		租户-参数配置
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page		query		int				false	"页码，默认 1"
//	@Param		page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param		keyword		query		string			false	"键名/名称关键字"
//	@Param		group_key	query		string			false	"按分组筛选"
//	@Success	200			{object}	response.Body	"data 为 response.Page[Item]"
//	@Router		/tenant/configs [get]
func (s *Service) handleTenantList(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	page, err := s.ListTenant(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, page)
}

// handleTenantCreate 租户新建参数
//
//	@Summary	租户新建参数
//	@Tags		租户-参数配置
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		Input			true	"参数内容"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Failure	409		{object}	response.Body	"参数键已存在"
//	@Router		/tenant/configs [post]
func (s *Service) handleTenantCreate(c *gin.Context) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.CreateTenant(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleTenantUpdate 租户编辑参数
//
//	@Summary	租户编辑参数
//	@Tags		租户-参数配置
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"参数 ID"
//	@Param		body	body		Input			true	"参数内容"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Router		/tenant/configs/{id} [put]
func (s *Service) handleTenantUpdate(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.UpdateTenant(c.Request.Context(), id, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleTenantDelete 租户删除参数
//
//	@Summary	租户删除参数
//	@Tags		租户-参数配置
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		int	true	"参数 ID"
//	@Success	200	{object}	response.Body
//	@Router		/tenant/configs/{id} [delete]
func (s *Service) handleTenantDelete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.DeleteTenant(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
