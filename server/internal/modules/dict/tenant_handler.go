package dict

import (
	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleTenantListDicts 租户字典类型分页列表
//
//	@Summary	租户字典类型列表
//	@Tags		租户-字典管理
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page		query		int				false	"页码，默认 1"
//	@Param		page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param		keyword		query		string			false	"编码/名称关键字"
//	@Success	200			{object}	response.Body	"data 为 response.Page[Item]"
//	@Router		/tenant/dicts [get]
func (s *Service) handleTenantListDicts(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	page, err := s.ListTenantDicts(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, page)
}

// handleTenantCreateDict 租户新建字典类型
//
//	@Summary	租户新建字典类型
//	@Tags		租户-字典管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		DictInput		true	"字典类型"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Failure	409		{object}	response.Body	"字典编码已存在"
//	@Router		/tenant/dicts [post]
func (s *Service) handleTenantCreateDict(c *gin.Context) {
	var in DictInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	item, err := s.CreateTenantDict(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleTenantUpdateDict 租户编辑字典类型
//
//	@Summary	租户编辑字典类型
//	@Tags		租户-字典管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"字典 ID"
//	@Param		body	body		DictInput		true	"字典类型"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Router		/tenant/dicts/{id} [put]
func (s *Service) handleTenantUpdateDict(c *gin.Context) {
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
	item, err := s.UpdateTenantDict(c.Request.Context(), id, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleTenantDeleteDict 租户删除字典类型
//
//	@Summary		租户删除字典类型
//	@Description	软删字典类型，其下字典项一并软删
//	@Tags			租户-字典管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"字典 ID"
//	@Success		200	{object}	response.Body
//	@Router			/tenant/dicts/{id} [delete]
func (s *Service) handleTenantDeleteDict(c *gin.Context) {
	id, err := pathID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.DeleteTenantDict(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleTenantListItems 租户字典项列表
//
//	@Summary		租户字典项列表
//	@Description	返回指定字典类型下的全部字典项（按 sort 排序）
//	@Tags			租户-字典管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int				true	"字典类型 ID"
//	@Success		200	{object}	response.Body	"data 为 []Entry"
//	@Router			/tenant/dicts/{id}/items [get]
func (s *Service) handleTenantListItems(c *gin.Context) {
	id, err := pathID(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}
	list, err := s.ListTenantItems(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// handleTenantCreateItem 租户新建字典项
//
//	@Summary	租户新建字典项
//	@Tags		租户-字典管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"字典类型 ID"
//	@Param		body	body		DictItemInput	true	"字典项"
//	@Success	200		{object}	response.Body	"data 为 Entry"
//	@Failure	409		{object}	response.Body	"同字典下 value 已存在"
//	@Router		/tenant/dicts/{id}/items [post]
func (s *Service) handleTenantCreateItem(c *gin.Context) {
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
	item, err := s.CreateTenantItem(c.Request.Context(), id, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleTenantUpdateItem 租户编辑字典项
//
//	@Summary	租户编辑字典项
//	@Tags		租户-字典管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"字典类型 ID"
//	@Param		itemId	path		int				true	"字典项 ID"
//	@Param		body	body		DictItemInput	true	"字典项"
//	@Success	200		{object}	response.Body	"data 为 Entry"
//	@Router		/tenant/dicts/{id}/items/{itemId} [put]
func (s *Service) handleTenantUpdateItem(c *gin.Context) {
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
	item, err := s.UpdateTenantItem(c.Request.Context(), dictID, itemID, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleTenantDeleteItem 租户删除字典项
//
//	@Summary	租户删除字典项
//	@Tags		租户-字典管理
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int	true	"字典类型 ID"
//	@Param		itemId	path		int	true	"字典项 ID"
//	@Success	200		{object}	response.Body
//	@Router		/tenant/dicts/{id}/items/{itemId} [delete]
func (s *Service) handleTenantDeleteItem(c *gin.Context) {
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
	if err := s.DeleteTenantItem(c.Request.Context(), dictID, itemID); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}
