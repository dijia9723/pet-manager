package tenant

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
	"pet-manager/server/internal/router"
)

// Name 实现 router.Module。
func (s *Service) Name() string { return "platform.tenant" }

// Register 挂载平台端租户管理接口。
// 平台分组统一按 user_type=platform + 权限点校验，租户身份无法触达。
func (s *Service) Register(r *router.Router) {
	g := r.Platform("/tenants")
	g.GET("", r.Require("platform.tenant.list"), s.handleList)
	g.POST("", r.Require("platform.tenant.create"), s.handleCreate)
	g.GET("/:id", r.Require("platform.tenant.list"), s.handleDetail)
	g.PUT("/:id", r.Require("platform.tenant.update"), s.handleUpdate)
	g.POST("/:id/enable", r.Require("platform.tenant.update"), s.handleEnable)
	g.POST("/:id/disable", r.Require("platform.tenant.update"), s.handleDisable)
	g.POST("/:id/delete", r.Require("platform.tenant.delete"), s.handleDelete)
	g.POST("/:id/restore", r.Require("platform.tenant.delete"), s.handleRestore)
	g.POST("/:id/destroy", r.Require("platform.tenant.destroy"), s.handleDestroy)
}

// handleList 租户分页列表
//
//	@Summary		租户列表
//	@Description	平台端分页查询租户，支持按名称/编码关键字筛选
//	@Tags			平台-租户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page		query		int				false	"页码，默认 1"
//	@Param			page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param			keyword		query		string			false	"名称/编码关键字"
//	@Param			status		query		string			false	"状态筛选 enabled/disabled"
//	@Success		200			{object}	response.Body	"data 为 response.Page[Item]"
//	@Failure		403			{object}	response.Body
//	@Router			/platform/tenants [get]
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

// handleDetail 租户详情
//
//	@Summary	租户详情
//	@Tags		平台-租户管理
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		int				true	"租户 ID"
//	@Success	200	{object}	response.Body	"data 为 Item"
//	@Failure	404	{object}	response.Body
//	@Router		/platform/tenants/{id} [get]
func (s *Service) handleDetail(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	item, err := s.Detail(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, item)
}

// handleCreate 开通租户
//
//	@Summary		开通租户
//	@Description	原子完成建租户记录→建 schema 建表→预置角色→建主账号；失败自动补偿回滚
//	@Tags			平台-租户管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		CreateInput		true	"租户与主账号信息"
//	@Success		200		{object}	response.Body	"data 为 Item"
//	@Failure		409		{object}	response.Body	"租户编码/账号已存在"
//	@Router			/platform/tenants [post]
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

// handleUpdate 编辑租户基础资料
//
//	@Summary	编辑租户
//	@Tags		平台-租户管理
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		int				true	"租户 ID"
//	@Param		body	body		UpdateInput		true	"待更新字段"
//	@Success	200		{object}	response.Body	"data 为 Item"
//	@Failure	404		{object}	response.Body
//	@Router		/platform/tenants/{id} [put]
func (s *Service) handleUpdate(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in UpdateInput
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

// handleEnable 启用租户
//
//	@Summary	启用租户
//	@Tags		平台-租户管理
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		int	true	"租户 ID"
//	@Success	200	{object}	response.Body
//	@Router		/platform/tenants/{id}/enable [post]
func (s *Service) handleEnable(c *gin.Context) { s.setStatus(c, model.StatusEnabled) }

// handleDisable 停用租户
//
//	@Summary		停用租户
//	@Description	停用后该租户全部会话立即失效，再次请求被租户门禁拦截
//	@Tags			平台-租户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"租户 ID"
//	@Success		200	{object}	response.Body
//	@Router			/platform/tenants/{id}/disable [post]
func (s *Service) handleDisable(c *gin.Context) { s.setStatus(c, model.StatusDisabled) }

// setStatus 解析路径 ID 后启停租户，供启用/停用两个 handler 复用。
func (s *Service) setStatus(c *gin.Context, status string) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.SetStatus(c.Request.Context(), id, status); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleDelete 删除租户（进入 7 天销毁宽限期）
//
//	@Summary		删除租户
//	@Description	仅标记待销毁并踢掉全部会话，不立即删除数据；按参数 tenant.purge_after_days（默认 7 天）到期后由系统定时任务物理销毁 schema，宽限期内可调用恢复接口撤销
//	@Tags			平台-租户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"租户 ID"
//	@Success		200	{object}	response.Body
//	@Failure		404	{object}	response.Body	"租户不存在"
//	@Router			/platform/tenants/{id}/delete [post]
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

// handleRestore 恢复待销毁租户
//
//	@Summary		恢复租户
//	@Description	撤销待销毁标记：清空销毁时间，租户恢复为停用状态，需手动启用；不在宽限期内的租户调用返回 409
//	@Tags			平台-租户管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"租户 ID"
//	@Success		200	{object}	response.Body
//	@Failure		409	{object}	response.Body	"租户未处于待销毁状态"
//	@Router			/platform/tenants/{id}/restore [post]
func (s *Service) handleRestore(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.Restore(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleDestroy godoc
// @Summary 立即永久销毁单个租户
// @Description 跳过宽限期，输入匹配的租户编码后立即删除数据库与账号，存储对象进入自动清理队列
// @Tags 平台-租户管理
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "租户 ID"
// @Param body body DestroyInput true "确认租户编码"
// @Success 200 {object} response.Body "data 为 DestroyResult"
// @Router /platform/tenants/{id}/destroy [post]
func (s *Service) handleDestroy(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var in DestroyInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, errcode.Wrap(errcode.ErrInvalidParam, err))
		return
	}
	result, err := s.Destroy(c.Request.Context(), id, in)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// pathID 解析并校验路径参数 :id。
func pathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}
