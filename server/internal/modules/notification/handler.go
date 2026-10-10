package notification

import (
	"github.com/gin-gonic/gin"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
	"strconv"
)

// handleManagedAttachment 下载发布治理视角下的通知附件，需发送方权限。
// handleManagedAttachment godoc
// @Summary 下载发布通知的附件
// @Tags 信息通知
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Param resource_id path int true "附件资源 ID"
// @Success 200 {object} response.Body
// @Router /platform/notifications/{id}/attachments/{resource_id}/download [get]
// @Router /tenant/notifications/{id}/attachments/{resource_id}/download [get]
func (s *Service) handleManagedAttachment(c *gin.Context) { s.handleAttachment(c, true) }

// handleReceivedAttachment 下载本人收件箱中未撤回消息的附件。
// handleReceivedAttachment godoc
// @Summary 下载本人收到且未撤回的消息附件
// @Tags 消息中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Param resource_id path int true "附件资源 ID"
// @Success 200 {object} response.Body
// @Router /platform/messages/{id}/attachments/{resource_id}/download [get]
// @Router /tenant/messages/{id}/attachments/{resource_id}/download [get]
func (s *Service) handleReceivedAttachment(c *gin.Context) { s.handleAttachment(c, false) }

// handleAttachment 是两类附件下载的公共实现，两者仅鉴权视角不同。
func (s *Service) handleAttachment(c *gin.Context, management bool) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	rid, err := strconv.ParseUint(c.Param("resource_id"), 10, 64)
	if err != nil || rid == 0 {
		respond(c, nil, errcode.New(errcode.ErrInvalidParam))
		return
	}
	data, err := s.DownloadAttachment(c.Request.Context(), id, rid, management)
	respond(c, data, err)
}

// pathID 解析路径参数 id，0 与非法值统一按参数错误处理。
func pathID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, errcode.New(errcode.ErrInvalidParam)
	}
	return id, nil
}

// bindQuery 绑定查询参数，失败时写入统一错误响应。
func bindQuery(c *gin.Context, in any) bool {
	if err := c.ShouldBindQuery(in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return false
	}
	return true
}

// bindJSON 绑定 JSON 请求体，失败时写入统一错误响应。
func bindJSON(c *gin.Context, in any) bool {
	if err := c.ShouldBindJSON(in); err != nil {
		response.Fail(c, errcode.Newf(errcode.ErrInvalidParam, "参数不合法: %v", err))
		return false
	}
	return true
}

// respond 统一出口：有错走 response.Fail，成功走 response.OK。
func respond(c *gin.Context, data any, err error) {
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, data)
}

// handleList 通知管理列表：平台看平台下发，租户仅看本租户发布，附接收/已读/已处理统计。
// handleList 通知管理
// @Summary 通知管理列表（含接收、已读、已处理统计）
// @Tags 信息通知
// @Description 平台查看平台下发消息；租户仅查看本租户发布消息。
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "标题关键词"
// @Param status query string false "draft/published/revoked"
// @Success 200 {object} response.Body "data 为 response.Page[Item]"
// @Router /platform/notifications [get]
// @Router /tenant/notifications [get]
func (s *Service) handleList(c *gin.Context) {
	var q ListQuery
	if !bindQuery(c, &q) {
		return
	}
	data, err := s.List(c.Request.Context(), q)
	respond(c, data, err)
}

// handleTargets 搜索可接收对象；租户端仅允许 tenant_users 且仅返回本租户启用账号。
// handleTargets 接收对象搜索
// @Summary 分页搜索可接收通知的租户或账号
// @Tags 信息通知
// @Description 租户端 type 仅允许 tenant_users，结果仅包含本租户启用账号。
// @Produce json
// @Security BearerAuth
// @Param type query string true "tenants/tenant_users/platform_users"
// @Param keyword query string false "关键词"
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Success 200 {object} response.Body "data 为 response.Page[Option]"
// @Router /platform/notifications/targets [get]
// @Router /tenant/notifications/targets [get]
func (s *Service) handleTargets(c *gin.Context) {
	var q TargetQuery
	if !bindQuery(c, &q) {
		return
	}
	data, err := s.Targets(c.Request.Context(), q)
	respond(c, data, err)
}

// handleCreate 新建通知草稿，发送方固定取登录上下文，忽略请求体中的伪造字段。
// handleCreate 保存草稿
// @Summary 新建通知草稿
// @Tags 信息通知
// @Description 租户端仅允许 own_tenant 或 tenant_users，发送方从登录上下文写入。
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body Input true "通知内容与接收范围"
// @Success 200 {object} response.Body "data 为 model.Notification"
// @Router /platform/notifications [post]
// @Router /tenant/notifications [post]
func (s *Service) handleCreate(c *gin.Context) {
	var in Input
	if !bindJSON(c, &in) {
		return
	}
	data, err := s.Save(c.Request.Context(), 0, in)
	respond(c, data, err)
}

// handleUpdate 编辑通知草稿：仅草稿可改，租户仅能改本租户草稿。
// handleUpdate 编辑草稿
// @Summary 编辑通知草稿
// @Tags 信息通知
// @Description 租户仅可编辑本租户草稿，接收账号须归属当前租户。
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Param body body Input true "通知内容与接收范围"
// @Success 200 {object} response.Body "data 为 model.Notification"
// @Router /platform/notifications/{id} [put]
// @Router /tenant/notifications/{id} [put]
func (s *Service) handleUpdate(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in Input
	if !bindJSON(c, &in) {
		return
	}
	data, err := s.Save(c.Request.Context(), id, in)
	respond(c, data, err)
}

// handlePublish 发布草稿并在同一事务内生成接收人快照，发布时重新校验接收范围。
// handlePublish 发布草稿
// @Summary 发布通知并生成接收人快照
// @Tags 信息通知
// @Description 租户仅可发布本租户草稿，发布时重新校验接收账号归属。
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Success 200 {object} response.Body
// @Router /platform/notifications/{id}/publish [post]
// @Router /tenant/notifications/{id}/publish [post]
func (s *Service) handlePublish(c *gin.Context) {
	id, err := pathID(c)
	if err == nil {
		err = s.Publish(c.Request.Context(), id)
	}
	respond(c, nil, err)
}

// handleRevoke 撤回已发布通知：保留历史回执，收件箱不再可见。
// handleRevoke 撤回通知
// @Summary 撤回已发布通知（保留历史回执）
// @Tags 信息通知
// @Description 租户仅可撤回本租户发布消息。
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Success 200 {object} response.Body
// @Router /platform/notifications/{id}/revoke [post]
// @Router /tenant/notifications/{id}/revoke [post]
func (s *Service) handleRevoke(c *gin.Context) {
	id, err := pathID(c)
	if err == nil {
		err = s.Revoke(c.Request.Context(), id)
	}
	respond(c, nil, err)
}

// handleReceipts 查看通知的接收回执与处理反馈，租户仅限本租户发布消息。
// handleReceipts 回执
// @Summary 查看通知接收回执与处理反馈
// @Tags 信息通知
// @Description 租户仅可查看本租户发布消息的回执。
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param state query string false "unread/read/pending/processed"
// @Param keyword query string false "租户名称/账号关键词"
// @Success 200 {object} response.Body "data 为 response.Page[model.NotificationReceipt]"
// @Router /platform/notifications/{id}/receipts [get]
// @Router /tenant/notifications/{id}/receipts [get]
func (s *Service) handleReceipts(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var q ListQuery
	if !bindQuery(c, &q) {
		return
	}
	data, err := s.Receipts(c.Request.Context(), id, q)
	respond(c, data, err)
}

// handleInbox 返回当前账号的消息中心列表。
// handleInbox 收件箱
// @Summary 当前账号的消息中心
// @Tags 消息中心
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码"
// @Param page_size query int false "每页条数"
// @Param keyword query string false "标题关键词"
// @Param state query string false "unread/read/pending/processed"
// @Success 200 {object} response.Body "data 为 response.Page[InboxItem]"
// @Router /platform/messages [get]
// @Router /tenant/messages [get]
func (s *Service) handleInbox(c *gin.Context) {
	var q ListQuery
	if !bindQuery(c, &q) {
		return
	}
	data, err := s.Inbox(c.Request.Context(), q)
	respond(c, data, err)
}

// handleUnread 返回当前账号的未读消息数。
// handleUnread 未读数
// @Summary 当前账号的未读消息数量
// @Tags 消息中心
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Body "data 为整数"
// @Router /platform/messages/unread-count [get]
// @Router /tenant/messages/unread-count [get]
func (s *Service) handleUnread(c *gin.Context) {
	data, err := s.Unread(c.Request.Context())
	respond(c, data, err)
}

// handleRead 将本人消息标记为已读（幂等）。
// handleRead 已读
// @Summary 标记本人消息已读（幂等）
// @Tags 消息中心
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Success 200 {object} response.Body
// @Router /platform/messages/{id}/read [post]
// @Router /tenant/messages/{id}/read [post]
func (s *Service) handleRead(c *gin.Context) {
	id, err := pathID(c)
	if err == nil {
		err = s.Acknowledge(c.Request.Context(), id, nil)
	}
	respond(c, nil, err)
}

// handleProcess 提交本人的处理结果，仅需处理的已发布通知可提交一次。
// handleProcess 处理通知
// @Summary 提交本人的通知处理结果
// @Tags 消息中心
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "通知 ID"
// @Param body body ProcessInput true "处理反馈"
// @Success 200 {object} response.Body
// @Router /platform/messages/{id}/process [post]
// @Router /tenant/messages/{id}/process [post]
func (s *Service) handleProcess(c *gin.Context) {
	id, err := pathID(c)
	if err != nil {
		respond(c, nil, err)
		return
	}
	var in ProcessInput
	if !bindJSON(c, &in) {
		return
	}
	err = s.Acknowledge(c.Request.Context(), id, &in.Feedback)
	respond(c, nil, err)
}
