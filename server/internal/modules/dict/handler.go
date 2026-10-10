package dict

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleListDicts 字典类型分页列表
//
//	@Summary	字典类型列表
//	@Tags		平台-字典管理
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page		query		int				false	"页码，默认 1"
//	@Param		page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param		keyword		query		string			false	"编码/名称关键字"
//	@Success	200			{object}	response.Body	"data 为 response.Page[Item]"
//	@Router		/platform/dicts [get]
func (s *Service) handleListDicts(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	page, err := s.ListDicts(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, page)
}

// handleCreateDict 新建字典类型
//
//	@Summary	新建字典类型
//	@Tags		平台-字典管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		DictInput		true	"字典类型"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Failure	409		{object}	response.Body	"字典编码已存在"
//	@Router		/platform/dicts [post]
func (s *Service) handleCreateDict(c *gin.Context) {
	var in DictInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.CreateDict(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleUpdateDict 编辑字典类型
//
//	@Summary	编辑字典类型
//	@Tags		平台-字典管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"字典 ID"
//	@Param		body	body		DictInput		true	"字典类型"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Router		/platform/dicts/{id} [put]
func (s *Service) handleUpdateDict(c *gin.Context) {
	id, err := pathID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in DictInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.UpdateDict(c.Request.Context(), id, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleDeleteDict 删除字典类型
//
//	@Summary		删除字典类型
//	@Description	软删字典类型，其下字典项一并软删
//	@Tags			平台-字典管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"字典 ID"
//	@Success		200	{object}	response.Body
//	@Router			/platform/dicts/{id} [delete]
func (s *Service) handleDeleteDict(c *gin.Context) {
	id, err := pathID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.DeleteDict(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleListItems 字典项列表
//
//	@Summary		字典项列表
//	@Description	返回指定字典类型下的全部字典项（按 sort 排序）
//	@Tags			平台-字典管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int				true	"字典类型 ID"
//	@Success		200	{object}	response.Body	"data 为 []Entry"
//	@Router			/platform/dicts/{id}/items [get]
func (s *Service) handleListItems(c *gin.Context) {
	id, err := pathID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	list, err := s.ListItems(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// handleCreateItem 新建字典项
//
//	@Summary	新建字典项
//	@Tags		平台-字典管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"字典类型 ID"
//	@Param		body	body		DictItemInput	true	"字典项"
//	@Success	200		{object}	response.Body	"data 为 Entry"
//	@Failure	409		{object}	response.Body	"同字典下 value 已存在"
//	@Router		/platform/dicts/{id}/items [post]
func (s *Service) handleCreateItem(c *gin.Context) {
	id, err := pathID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in DictItemInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.CreateItem(c.Request.Context(), id, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleUpdateItem 编辑字典项
//
//	@Summary	编辑字典项
//	@Tags		平台-字典管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"字典类型 ID"
//	@Param		itemId	path		int				true	"字典项 ID"
//	@Param		body	body		DictItemInput	true	"字典项"
//	@Success	200		{object}	response.Body	"data 为 Entry"
//	@Router		/platform/dicts/{id}/items/{itemId} [put]
func (s *Service) handleUpdateItem(c *gin.Context) {
	dictID, err := pathID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	itemID, err := pathID(c, "itemId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in DictItemInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.UpdateItem(c.Request.Context(), dictID, itemID, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleDeleteItem 删除字典项
//
//	@Summary	删除字典项
//	@Tags		平台-字典管理
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int	true	"字典类型 ID"
//	@Param		itemId	path		int	true	"字典项 ID"
//	@Success	200		{object}	response.Body
//	@Router		/platform/dicts/{id}/items/{itemId} [delete]
func (s *Service) handleDeleteItem(c *gin.Context) {
	dictID, err := pathID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	itemID, err := pathID(c, "itemId")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.DeleteItem(c.Request.Context(), dictID, itemID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// pathID 解析指定名称的路径参数，非正整数一律按参数不合法处理。
func pathID(c *gin.Context, name string) (uint64, error) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}
