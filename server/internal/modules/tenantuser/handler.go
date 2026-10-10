package tenantuser

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleList 子账号分页列表
//
//	@Summary		子账号列表
//	@Description	当前租户下的子账号分页列表，含角色 ID 与角色名
//	@Tags			租户-子账号管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int				false	"页码，默认 1"
//	@Param			page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param			keyword		query		string			false	"用户名/昵称关键字"
//	@Success		200			{object}	response.Body	"data 为 response.Page[Item]"
//	@Router			/tenant/users [get]
func (s *Service) handleList(c *gin.Context) {
	var q ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, errcode.New(errcode.ErrInvalidParam))
		return
	}
	page, err := s.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, page)
}

// handleCreate 新建子账号
//
//	@Summary		新建子账号
//	@Description	建账号与角色绑定在同一租户事务内原子完成
//	@Tags			租户-子账号管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		CreateInput		true	"子账号信息"
//	@Success		200		{object}	response.Body	"data 为 Item"
//	@Failure		409		{object}	response.Body	"账号已存在"
//	@Router			/tenant/users [post]
func (s *Service) handleCreate(c *gin.Context) {
	var in CreateInput
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

// handleUpdate 编辑子账号
//
//	@Summary		编辑子账号
//	@Description	修改昵称/状态/角色绑定；主账号受保护不可停用；改绑后立即失效权限缓存
//	@Tags			租户-子账号管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int			true	"子账号 ID"
//	@Param			body	body		UpdateInput	true	"待更新字段"
//	@Success		200		{object}	response.Body
//	@Router			/tenant/users/{id} [put]
func (s *Service) handleUpdate(c *gin.Context) {
	id, err := userPathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in UpdateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	if err := s.Update(c.Request.Context(), id, in); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleDelete 删除子账号
//
//	@Summary		删除子账号
//	@Description	软删账号并解除角色绑定；不能删除自己；租户主账号受保护
//	@Tags			租户-子账号管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"子账号 ID"
//	@Success		200	{object}	response.Body
//	@Router			/tenant/users/{id} [delete]
func (s *Service) handleDelete(c *gin.Context) {
	id, err := userPathID(c)
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

// handleResetPassword 重置子账号密码
//
//	@Summary		重置密码
//	@Description	重置后踢掉该账号全部会话，需重新登录
//	@Tags			租户-子账号管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int					true	"子账号 ID"
//	@Param			body	body		ResetPasswordInput	true	"新密码"
//	@Success		200		{object}	response.Body
//	@Router			/tenant/users/{id}/reset-password [post]
func (s *Service) handleResetPassword(c *gin.Context) {
	id, err := userPathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in ResetPasswordInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	if err := s.ResetPassword(c.Request.Context(), id, in); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// userPathID 解析并校验路径参数 :id。
func userPathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}
