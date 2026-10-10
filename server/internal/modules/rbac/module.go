package rbac

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "tenant.rbac" }

// Register 挂载租户端角色与权限点接口。
// Tenant 分组已内置"登录 + 租户门禁 + 租户事务"，这里只声明权限点。
func (s *Service) Register(r *router.Router) {
	g := r.Tenant("")
	g.GET("/roles", r.Require("tenant.role.list"), s.handleListRoles)
	g.POST("/roles", r.Require("tenant.role.create"), s.handleCreateRole)
	g.PUT("/roles/:id", r.Require("tenant.role.update"), s.handleUpdateRole)
	g.DELETE("/roles/:id", r.Require("tenant.role.delete"), s.handleDeleteRole)
	g.GET("/permissions", r.Require("tenant.role.list"), s.handlePermissionTree)
}

// handleListRoles 角色列表
//
//	@Summary		角色列表
//	@Description	返回当前租户全部角色（含预置主账号/店员角色）及其权限点 ID
//	@Tags			租户-角色授权
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Body	"data 为 []RoleItem"
//	@Router			/tenant/roles [get]
func (s *Service) handleListRoles(c *gin.Context) {
	list, err := s.ListRoles(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// handleCreateRole 新建角色
//
//	@Summary	新建角色
//	@Tags		租户-角色授权
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		body	body		RoleInput		true	"角色与授权权限点"
//	@Success	200		{object}	response.Body	"data 为 RoleItem"
//	@Failure	409		{object}	response.Body	"角色编码已存在"
//	@Router		/tenant/roles [post]
func (s *Service) handleCreateRole(c *gin.Context) {
	var in RoleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	id, err := s.CreateRole(c.Request.Context(), in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	item, err := s.roleItem(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleUpdateRole 编辑角色与授权
//
//	@Summary		编辑角色
//	@Description	修改名称/备注/权限点；预置角色的 code 不可改
//	@Tags			租户-角色授权
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int				true	"角色 ID"
//	@Param			body	body		RoleInput		true	"角色与授权权限点"
//	@Success		200		{object}	response.Body	"data 为 RoleItem"
//	@Router			/tenant/roles/{id} [put]
func (s *Service) handleUpdateRole(c *gin.Context) {
	id, err := rolePathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in RoleInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return
	}
	if err := s.UpdateRole(c.Request.Context(), id, in); err != nil {
		response.Fail(c, err)
		return
	}
	item, err := s.roleItem(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleDeleteRole 删除角色
//
//	@Summary		删除角色
//	@Description	预置角色禁止删除；仍被子账号引用时返回 20015 错误码
//	@Tags			租户-角色授权
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"角色 ID"
//	@Success		200	{object}	response.Body
//	@Failure		409	{object}	response.Body	"角色已被账号引用"
//	@Router			/tenant/roles/{id} [delete]
func (s *Service) handleDeleteRole(c *gin.Context) {
	id, err := rolePathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.DeleteRole(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handlePermissionTree 当前租户可选的权限点树
//
//	@Summary		权限点树
//	@Description	返回平台下发、当前租户身份可见的权限点树，供角色授权页勾选
//	@Tags			租户-角色授权
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Body	"data 为 []PermissionNode"
//	@Router			/tenant/permissions [get]
func (s *Service) handlePermissionTree(c *gin.Context) {
	tree, err := s.PermissionTree(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, tree)
}

// roleItem 复用列表查询取单条，避免为"写后读"再写一份组装逻辑。
func (s *Service) roleItem(ctx context.Context, id uint64) (*RoleItem, error) {
	list, err := s.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, errcode.New(errcode.ErrNotFound)
}

// rolePathID 解析并校验路径参数 :id。
func rolePathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}
