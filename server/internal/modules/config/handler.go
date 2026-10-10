package config

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleList 参数配置分页列表
//
//	@Summary	参数列表
//	@Tags		平台-参数配置
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page		query		int				false	"页码，默认 1"
//	@Param		page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param		keyword		query		string			false	"键名/名称关键字"
//	@Param		group_key	query		string			false	"按分组筛选"
//	@Success	200			{object}	response.Body	"data 为 response.Page[Item]"
//	@Router		/platform/configs [get]
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

// handleCreate 新建参数
//
//	@Summary	新建参数
//	@Tags		平台-参数配置
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		Input			true	"参数内容"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Failure	409		{object}	response.Body	"参数键已存在"
//	@Router		/platform/configs [post]
func (s *Service) handleCreate(c *gin.Context) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.Create(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleUpdate 编辑参数
//
//	@Summary	编辑参数
//	@Tags		平台-参数配置
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"参数 ID"
//	@Param		body	body		Input			true	"参数内容"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Router		/platform/configs/{id} [put]
func (s *Service) handleUpdate(c *gin.Context) {
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
	item, err := s.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleDelete 删除参数
//
//	@Summary	删除参数
//	@Tags		平台-参数配置
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		int	true	"参数 ID"
//	@Success	200	{object}	response.Body
//	@Router		/platform/configs/{id} [delete]
func (s *Service) handleDelete(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// pathID 解析路径参数 id，非正整数一律按参数不合法处理。
func pathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}
