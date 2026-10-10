package permission

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleTree 权限点菜单树
//
//	@Summary		权限点树
//	@Description	返回菜单配置页的完整权限点树（含 menu/button 两类节点）
//	@Tags			平台-菜单配置
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Body	"data 为 []Node"
//	@Router			/platform/permissions/tree [get]
func (s *Service) handleTree(c *gin.Context) {
	tree, err := s.Tree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, tree)
}

// handleCreate 新建权限点
//
//	@Summary		新建权限点
//	@Description	新建后全量失效所有租户的权限缓存；写操作需平台权限点
//	@Tags			平台-菜单配置
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		Input			true	"权限点定义"
//	@Success		200		{object}	response.Body	"data 为 Node"
//	@Router			/platform/permissions [post]
func (s *Service) handleCreate(c *gin.Context) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	node, err := s.Create(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// handleUpdate 编辑权限点
//
//	@Summary		编辑权限点
//	@Description	名称/路径/图标/排序等治理项保存在线配置；保存后全量失效权限缓存
//	@Tags			平台-菜单配置
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int				true	"权限点 ID"
//	@Param			body	body		Input			true	"权限点定义"
//	@Success		200		{object}	response.Body	"data 为 Node"
//	@Router			/platform/permissions/{id} [put]
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
	node, err := s.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// handleDelete 删除权限点
//
//	@Summary		删除权限点
//	@Description	软删权限点并解除角色引用，保存后全量失效权限缓存
//	@Tags			平台-菜单配置
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"权限点 ID"
//	@Success		200	{object}	response.Body
//	@Router			/platform/permissions/{id} [delete]
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

// pathID 解析并校验路径参数 :id。
func pathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}
