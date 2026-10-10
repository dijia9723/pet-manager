package job

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// handleMethods 可供新建系统任务时选择执行方法，并返回当前主机 shell 是否可用。
//
//	@Summary	可调用的系统任务方法
//	@Tags		平台-任务管理
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	response.Body{data=MethodCatalog}
//	@Router		/platform/jobs/methods [get]
func (s *Service) handleMethods(c *gin.Context) { response.OK(c, s.Methods()) }

// handleCreate 新建自定义任务
//
//	@Summary		新建定时任务
//	@Description	支持 system（已注册方法）、http（HTTP请求或curl）、shell（sh脚本）；默认停用
//	@Tags			平台-任务管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			body	body		CreateInput	true	"任务配置"
//	@Success		200		{object}	response.Body{data=Item}
//	@Failure		400		{object}	response.Body
//	@Failure		409		{object}	response.Body
//	@Router			/platform/jobs [post]
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

// handleDelete 删除自定义任务
//
//	@Summary		删除自定义任务
//	@Description	仅允许删除已停用且未执行中的自定义任务，系统预置任务受保护
//	@Tags			平台-任务管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"任务ID"
//	@Success		200	{object}	response.Body
//	@Failure		403	{object}	response.Body
//	@Failure		409	{object}	response.Body
//	@Router			/platform/jobs/{id} [delete]
func (s *Service) handleDelete(c *gin.Context) {
	id, err := jobPathID(c)
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

// handleList 任务列表
//
//	@Summary		定时任务列表
//	@Description	平台侧查询全部系统定时任务及其最近/下次执行状态
//	@Tags			平台-任务管理
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	response.Body	"data 为 []Item"
//	@Router			/platform/jobs [get]
func (s *Service) handleList(c *gin.Context) {
	items, err := s.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

// handleUpdate 编辑任务调度参数
//
//	@Summary		编辑定时任务
//	@Description	修改任务名称、类型、执行配置、超时、5 段 cron 与备注；预置任务仅允许修改调度、超时和备注
//	@Tags			平台-任务管理
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		int			true	"任务 ID"
//	@Param			body	body		UpdateInput	true	"调度参数"
//	@Success		200		{object}	response.Body
//	@Failure		400		{object}	response.Body	"cron 表达式非法"
//	@Failure		404		{object}	response.Body	"任务不存在"
//	@Router			/platform/jobs/{id} [put]
func (s *Service) handleUpdate(c *gin.Context) {
	id, err := jobPathID(c)
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

// handleEnable 启用任务
//
//	@Summary	启用定时任务
//	@Tags		平台-任务管理
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		int	true	"任务 ID"
//	@Success	200	{object}	response.Body
//	@Router		/platform/jobs/{id}/enable [post]
func (s *Service) handleEnable(c *gin.Context) {
	id, err := jobPathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.SetEnabled(c.Request.Context(), id, true); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleDisable 停用任务
//
//	@Summary		停用定时任务
//	@Description	停用后不再调度，已在执行中的任务不受影响
//	@Tags			平台-任务管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"任务 ID"
//	@Success		200	{object}	response.Body
//	@Router			/platform/jobs/{id}/disable [post]
func (s *Service) handleDisable(c *gin.Context) {
	id, err := jobPathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.SetEnabled(c.Request.Context(), id, false); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleRun 立即执行一次
//
//	@Summary		立即执行任务
//	@Description	异步触发一次手动执行，不影响原有调度；任务正在执行时返回 409，结果在执行日志中查看
//	@Tags			平台-任务管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		int	true	"任务 ID"
//	@Success		200	{object}	response.Body
//	@Failure		409	{object}	response.Body	"任务正在执行中"
//	@Router			/platform/jobs/{id}/run [post]
func (s *Service) handleRun(c *gin.Context) {
	id, err := jobPathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if err := s.Trigger(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nil)
}

// handleLogs 任务执行日志
//
//	@Summary		任务执行日志
//	@Description	分页查询单个任务的执行历史，支持按执行状态/触发方式筛选
//	@Tags			平台-任务管理
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		int				true	"任务 ID"
//	@Param			page		query		int				false	"页码，默认 1"
//	@Param			page_size	query		int				false	"每页条数，默认 20，最大 200"
//	@Param			status		query		string			false	"状态筛选 running/success/fail"
//	@Param			trigger		query		string			false	"触发方式 schedule/manual"
//	@Success		200			{object}	response.Body	"data 为 response.Page[LogItem]"
//	@Router			/platform/jobs/{id}/logs [get]
func (s *Service) handleLogs(c *gin.Context) {
	id, err := jobPathID(c)
	if err != nil {
		response.Fail(c, err)
		return
	}
	var q LogListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, errcode.New(errcode.ErrInvalidParam))
		return
	}
	page, err := s.ListLogs(c.Request.Context(), id, q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, page)
}

// jobPathID 解析并校验路径参数 :id。
func jobPathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}
