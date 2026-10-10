// Package notification 提供平台下发、租户店内发布及账号级回执。租户请求使用请求事务，
// 并显式限定平台表 schema 和当前接收人，避免 search_path 或账号 ID 重叠导致越权。
package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/modules/storage"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
)

// Service 负责通知草稿、发布快照、回执与消息中心。
type Service struct {
	manager *db.Manager      // 平台库管理器
	storage *storage.Service // 附件校验与下载，nil 表示未配置存储
}

// NewService 构造通知服务；可选传入 storage 服务以支持附件。
func NewService(m *db.Manager, stores ...*storage.Service) *Service {
	s := &Service{manager: m}
	if len(stores) > 0 {
		s.storage = stores[0]
	}
	return s
}

// Input 是新建或编辑通知的入参；发送方不在此结构内，一律取自登录上下文。
type Input struct {
	AttachmentIDs  []uint64 `json:"attachment_ids" binding:"max=10,dive,gt=0"` // 引用本存储空间的就绪资源
	Title          string   `json:"title" binding:"required,max=128"`
	Content        string   `json:"content" binding:"required,max=20000"`
	Category       string   `json:"category" binding:"required,oneof=announcement maintenance business"`
	Priority       string   `json:"priority" binding:"required,oneof=normal urgent"`
	RequireProcess bool     `json:"require_process"` // 是否需要接收人提交处理结果
	TargetType     string   `json:"target_type" binding:"required,oneof=all_tenants tenants tenant_users platform_users own_tenant"`
	TargetIDs      []uint64 `json:"target_ids" binding:"max=500,dive,gt=0"`
}

// ProcessInput 是接收人提交的处理反馈。
type ProcessInput struct {
	Feedback string `json:"feedback" binding:"required,max=2000"`
}

// ListQuery 是通知管理、回执与收件箱共用的查询参数。
type ListQuery struct {
	pagination.Query
	Keyword string `form:"keyword"`
	Status  string `form:"status" binding:"omitempty,oneof=draft published revoked"`
	State   string `form:"state" binding:"omitempty,oneof=unread read pending processed"`
}

// Item 是通知管理列表项，附带接收/已读/已处理统计。
type Item struct {
	model.Notification
	RecipientCount int64 `json:"recipient_count"`
	ReadCount      int64 `json:"read_count"`
	ProcessedCount int64 `json:"processed_count"`
}

// InboxItem 是消息中心列表项，附带当前账号的已读与处理状态。
type InboxItem struct {
	model.Notification
	ReadAt      *time.Time `json:"read_at"`
	ProcessedAt *time.Time `json:"processed_at"`
	Feedback    string     `json:"feedback"`
}

// Option 是接收对象搜索的候选项。
type Option struct {
	ID    uint64 `json:"id"`
	Label string `json:"label"`
}

// TargetQuery 是接收对象搜索参数。
type TargetQuery struct {
	pagination.Query
	Type    string `form:"type" binding:"required,oneof=tenants tenant_users platform_users"`
	Keyword string `form:"keyword"`
}

// validateInput 规范化标题/内容，去重目标 ID，并校验接收范围与正文非空。
func validateInput(in *Input) error {
	in.Title, in.Content = strings.TrimSpace(in.Title), strings.TrimSpace(in.Content)
	if in.Title == "" || in.Content == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "标题和内容不能为空")
	}
	if in.TargetType == "all_tenants" || in.TargetType == "own_tenant" {
		in.TargetIDs = []uint64{}
	} else if len(in.TargetIDs) == 0 {
		return errcode.Newf(errcode.ErrInvalidParam, "请选择接收对象")
	}
	seen := map[uint64]bool{}
	ids := make([]uint64, 0, len(in.TargetIDs))
	for _, id := range in.TargetIDs {
		if id == 0 {
			return errcode.New(errcode.ErrInvalidParam)
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	in.TargetIDs = ids
	return nil
}

// table 拼接平台 schema 下的限定表名，避免依赖 search_path。
func (s *Service) table(name string) string {
	return fmt.Sprintf("%q.%q", s.manager.PlatformSchema(), name)
}

// dbError 把 GORM 错误映射为统一业务错误码。
func dbError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.New(errcode.ErrNotFound)
	}
	if err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	return nil
}

// applyInput 把入参写入通知实体，发送方字段由调用方单独设置。
func applyInput(n *model.Notification, in Input) {
	n.Title, n.Content, n.Category, n.Priority = in.Title, in.Content, in.Category, in.Priority
	n.RequireProcess, n.TargetType, n.TargetIDs = in.RequireProcess, in.TargetType, in.TargetIDs
}

// 通知与账号表均显式限定平台 schema，租户端只使用请求事务。
// notifications 返回平台 schema 下的通知查询。
func (s *Service) notifications(tx *gorm.DB) *gorm.DB {
	return tx.Model(&model.Notification{}).Table(s.table("sys_notification"))
}

// receipts 返回平台 schema 下的接收回执查询。
func (s *Service) receipts(tx *gorm.DB) *gorm.DB {
	return tx.Model(&model.NotificationReceipt{}).Table(s.table("sys_notification_receipt"))
}

// managed 限定为当前调用方自己发布的通知，作为治理视角的越权护栏。
func (s *Service) managed(tx *gorm.DB, ctx context.Context) *gorm.DB {
	return s.notifications(tx).Where("sender_type = ? AND sender_tenant_id = ?", ctxkey.UserType(ctx), ctxkey.TenantID(ctx))
}

// validateScope 校验接收范围：平台不得选 own_tenant，租户只能发本租户账号。
func (s *Service) validateScope(tx *gorm.DB, ctx context.Context, in Input) error {
	if ctxkey.UserType(ctx) == model.UserTypePlatform {
		if in.TargetType == "own_tenant" {
			return errcode.New(errcode.ErrInvalidParam)
		}
		return nil
	}
	if in.TargetType != "own_tenant" && in.TargetType != "tenant_users" {
		return errcode.Newf(errcode.ErrForbiddenAction, "租户只能向本租户账号发布消息")
	}
	if in.TargetType == "tenant_users" {
		var count int64
		if err := tx.Model(&model.TenantUser{}).Table(s.table("sys_tenant_user")).Where("id IN ? AND tenant_id = ? AND status = ?", in.TargetIDs, ctxkey.TenantID(ctx), model.StatusEnabled).Count(&count).Error; err != nil {
			return dbError(err)
		}
		if count != int64(len(in.TargetIDs)) {
			return errcode.Newf(errcode.ErrForbiddenAction, "接收账号必须是本租户的启用账号")
		}
	}
	return nil
}

// Save 新建或编辑草稿：id 为 0 时新建，否则仅允许修改仍处于 draft 的通知。
func (s *Service) Save(ctx context.Context, id uint64, in Input) (*model.Notification, error) {
	if err := validateInput(&in); err != nil {
		return nil, err
	}
	var n model.Notification
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return nil, err
	}
	err = tx.Transaction(func(tx *gorm.DB) error {
		if err := s.validateScope(tx, ctx, in); err != nil {
			return err
		}
		if id != 0 {
			// 行锁串行化编辑，避免与发布并发时用到过期草稿。
			if err := s.managed(tx, ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&n, id).Error; err != nil {
				return dbError(err)
			}
			if n.Status != "draft" {
				return errcode.Newf(errcode.ErrConflict, "仅草稿允许编辑")
			}
		} else {
			n.Status, n.SenderID, n.SenderName = "draft", ctxkey.UserID(ctx), ctxkey.Username(ctx)
			n.SenderType, n.SenderTenantID = ctxkey.UserType(ctx), ctxkey.TenantID(ctx)
		}
		applyInput(&n, in)
		attachments, err := s.validateAttachments(ctx, tx, in.AttachmentIDs)
		if err != nil {
			return err
		}
		n.Attachments = attachments
		return dbError(tx.Table(s.table("sys_notification")).Save(&n).Error)
	})
	return &n, err
}

// Publish 与 Edit/Revoke 共用通知行锁；接收快照和状态在同一事务提交，重复发布返回冲突。
func (s *Service) Publish(ctx context.Context, id uint64) error {
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return err
	}
	return tx.Transaction(func(tx *gorm.DB) error {
		var n model.Notification
		if err := s.managed(tx, ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&n, id).Error; err != nil {
			return dbError(err)
		}
		if n.Status != "draft" {
			return errcode.Newf(errcode.ErrConflict, "仅草稿允许发布")
		}
		ids := make([]uint64, 0, len(n.Attachments))
		for _, a := range n.Attachments {
			ids = append(ids, a.ResourceID)
		}
		if _, err := s.validateAttachments(ctx, tx, ids); err != nil {
			return err
		}
		rows, err := s.resolveRecipients(tx, n)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return errcode.Newf(errcode.ErrInvalidParam, "没有可接收通知的启用账号")
		}
		if err := tx.Table(s.table("sys_notification_receipt")).CreateInBatches(&rows, 500).Error; err != nil {
			return dbError(err)
		}
		now := time.Now()
		return dbError(s.notifications(tx).Where("id = ?", n.ID).Updates(map[string]any{"status": "published", "published_at": now}).Error)
	})
}

// resolveRecipients 按目标范围解析接收人快照，任一目标失效即整体报错，不产生部分回执。
func (s *Service) resolveRecipients(tx *gorm.DB, n model.Notification) ([]model.NotificationReceipt, error) {
	rows := []model.NotificationReceipt{}
	if n.SenderType == model.UserTypeTenant && (n.SenderTenantID == 0 || (n.TargetType != "own_tenant" && n.TargetType != "tenant_users")) {
		return nil, errcode.Newf(errcode.ErrForbiddenAction, "租户只能向本租户账号发布消息")
	}
	if n.TargetType == "platform_users" {
		var users []model.PlatformUser
		if err := tx.Table(s.table("sys_platform_user")).Where("id IN ? AND status = ?", n.TargetIDs, model.StatusEnabled).Find(&users).Error; err != nil {
			return nil, dbError(err)
		}
		if len(users) != len(n.TargetIDs) {
			return nil, errcode.Newf(errcode.ErrInvalidParam, "接收账号不存在或已停用，请重新选择")
		}
		for _, u := range users {
			rows = append(rows, model.NotificationReceipt{NotificationID: n.ID, UserType: model.UserTypePlatform, UserID: u.ID, Username: u.Username})
		}
		return rows, nil
	}
	var tenants []model.Tenant
	tq := tx.Table(s.table("sys_tenant")).Where("status = ? AND (expire_at IS NULL OR expire_at > ?)", model.StatusEnabled, time.Now())
	if n.SenderType == model.UserTypeTenant {
		tq = tq.Where("id = ?", n.SenderTenantID)
	}
	if n.TargetType == "tenants" {
		tq = tq.Where("id IN ?", n.TargetIDs)
	}
	if err := tq.Find(&tenants).Error; err != nil {
		return nil, dbError(err)
	}
	if n.TargetType == "tenants" && len(tenants) != len(n.TargetIDs) {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "接收租户不存在、已停用或已到期，请重新选择")
	}
	names := map[uint64]string{}
	ids := []uint64{}
	for _, t := range tenants {
		names[t.ID] = t.Name
		ids = append(ids, t.ID)
	}
	var users []model.TenantUser
	uq := tx.Table(s.table("sys_tenant_user")).Where("tenant_id IN ? AND status = ?", ids, model.StatusEnabled)
	if n.TargetType == "tenant_users" {
		uq = uq.Where("id IN ?", n.TargetIDs)
	}
	if err := uq.Find(&users).Error; err != nil {
		return nil, dbError(err)
	}
	if n.TargetType == "tenant_users" && len(users) != len(n.TargetIDs) {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "接收账号或所属租户不可用，请重新选择")
	}
	for _, u := range users {
		rows = append(rows, model.NotificationReceipt{NotificationID: n.ID, UserType: model.UserTypeTenant, TenantID: u.TenantID, UserID: u.ID, Username: u.Username, TenantName: names[u.TenantID]})
	}
	return rows, nil
}

// Revoke 撤回已发布通知，仅更新状态并保留历史回执。
func (s *Service) Revoke(ctx context.Context, id uint64) error {
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return err
	}
	var n model.Notification
	if err := s.managed(tx, ctx).First(&n, id).Error; err != nil {
		return dbError(err)
	}
	res := s.managed(tx, ctx).Where("id = ? AND status = ?", id, "published").Update("status", "revoked")
	if res.Error != nil {
		return dbError(res.Error)
	}
	if res.RowsAffected == 0 {
		return errcode.Newf(errcode.ErrConflict, "仅已发布通知允许撤回")
	}
	return nil
}

// List 通知管理列表：仅返回当前调用方发布的通知，并聚合接收/已读/已处理统计。
func (s *Service) List(ctx context.Context, in ListQuery) (*response.Page[Item], error) {
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return nil, err
	}
	q := s.managed(tx, ctx)
	if in.Status != "" {
		q = q.Where("status = ?", in.Status)
	}
	if kw := strings.TrimSpace(in.Keyword); kw != "" {
		q = q.Where("title ILIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, dbError(err)
	}
	var ns []model.Notification
	if err := q.Order("id DESC").Limit(in.Limit()).Offset(in.Offset()).Find(&ns).Error; err != nil {
		return nil, dbError(err)
	}
	items := make([]Item, 0, len(ns))
	ids := make([]uint64, 0, len(ns))
	for _, n := range ns {
		ids = append(ids, n.ID)
	}
	type counts struct {
		NotificationID                            uint64
		RecipientCount, ReadCount, ProcessedCount int64
	}
	var cs []counts
	if len(ids) > 0 {
		q := s.receipts(tx).Select("notification_id, count(*) AS recipient_count, count(read_at) AS read_count, count(processed_at) AS processed_count").Where("notification_id IN ?", ids)
		if ctxkey.UserType(ctx) == model.UserTypeTenant {
			q = q.Where("tenant_id = ? AND user_type = ?", ctxkey.TenantID(ctx), model.UserTypeTenant)
		}
		err := q.Group("notification_id").Scan(&cs).Error
		if err != nil {
			return nil, dbError(err)
		}
	}
	cm := map[uint64]counts{}
	for _, c := range cs {
		cm[c.NotificationID] = c
	}
	for _, n := range ns {
		c := cm[n.ID]
		items = append(items, Item{n, c.RecipientCount, c.ReadCount, c.ProcessedCount})
	}
	return response.NewPage(items, total), nil
}

// Receipts 查询指定通知的回执；租户仅能看到本租户接收人的记录。
func (s *Service) Receipts(ctx context.Context, id uint64, in ListQuery) (*response.Page[model.NotificationReceipt], error) {
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return nil, err
	}
	var n model.Notification
	if err := s.managed(tx, ctx).First(&n, id).Error; err != nil {
		return nil, dbError(err)
	}
	q := s.receipts(tx).Where("notification_id = ?", id)
	if ctxkey.UserType(ctx) == model.UserTypeTenant {
		q = q.Where("tenant_id = ? AND user_type = ?", ctxkey.TenantID(ctx), model.UserTypeTenant)
	}
	q = receiptState(q, in.State, n.RequireProcess)
	if kw := strings.TrimSpace(in.Keyword); kw != "" {
		q = q.Where("username ILIKE ? OR tenant_name ILIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, dbError(err)
	}
	var rows []model.NotificationReceipt
	if err := q.Order("id ASC").Limit(in.Limit()).Offset(in.Offset()).Find(&rows).Error; err != nil {
		return nil, dbError(err)
	}
	return response.NewPage(rows, total), nil
}

// receiptState 按读取/处理状态过滤回执；未要求处理的待办恒为空集。
func receiptState(q *gorm.DB, state string, required bool) *gorm.DB {
	switch state {
	case "unread":
		return q.Where("read_at IS NULL")
	case "read":
		return q.Where("read_at IS NOT NULL")
	case "processed":
		return q.Where("processed_at IS NOT NULL")
	case "pending":
		if !required {
			return q.Where("1 = 0")
		}
		return q.Where("processed_at IS NULL")
	}
	return q
}

// inboxDB 按调用方身份返回数据库句柄：租户用请求事务，平台用平台库。
func (s *Service) inboxDB(ctx context.Context) (*gorm.DB, error) {
	if ctxkey.UserType(ctx) == model.UserTypeTenant {
		if ctxkey.TenantID(ctx) == 0 {
			return nil, errcode.New(errcode.ErrForbiddenAction)
		}
		return ctxkey.MustTenantDB(ctx)
	}
	if ctxkey.UserType(ctx) != model.UserTypePlatform {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}
	return s.manager.Platform().WithContext(ctx), nil
}

// own 追加当前账号接收快照的归属条件（类型+租户+账号）。
func (s *Service) own(q *gorm.DB, ctx context.Context) *gorm.DB {
	return q.Where("r.user_type = ? AND r.tenant_id = ? AND r.user_id = ?", ctxkey.UserType(ctx), ctxkey.TenantID(ctx), ctxkey.UserID(ctx))
}

// inboxQuery 关联接收快照与已发布通知，仅暴露本人可见的收件项。
func (s *Service) inboxQuery(tx *gorm.DB, ctx context.Context) *gorm.DB {
	q := tx.Table(s.table("sys_notification_receipt")+" AS r").Joins("JOIN "+s.table("sys_notification")+" AS n ON n.id = r.notification_id AND n.deleted_at IS NULL AND n.status = ?", "published").Where("r.deleted_at IS NULL")
	return s.own(q, ctx)
}

// Inbox 返回当前账号的消息中心列表，支持按状态与关键词过滤。
func (s *Service) Inbox(ctx context.Context, in ListQuery) (*response.Page[InboxItem], error) {
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return nil, err
	}
	q := s.inboxQuery(tx, ctx)
	if kw := strings.TrimSpace(in.Keyword); kw != "" {
		q = q.Where("n.title ILIKE ?", "%"+kw+"%")
	}
	switch in.State {
	case "unread":
		q = q.Where("r.read_at IS NULL")
	case "read":
		q = q.Where("r.read_at IS NOT NULL")
	case "pending":
		q = q.Where("n.require_process = TRUE AND r.processed_at IS NULL")
	case "processed":
		q = q.Where("r.processed_at IS NOT NULL")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, dbError(err)
	}
	var rows []InboxItem
	if err := q.Select("n.*, r.read_at, r.processed_at, r.feedback").Order("n.published_at DESC, n.id DESC").Limit(in.Limit()).Offset(in.Offset()).Scan(&rows).Error; err != nil {
		return nil, dbError(err)
	}
	return response.NewPage(rows, total), nil
}

// Unread 统计当前账号未读消息数。
func (s *Service) Unread(ctx context.Context) (int64, error) {
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	err = s.inboxQuery(tx, ctx).Where("r.read_at IS NULL").Count(&count).Error
	return count, dbError(err)
}

// Acknowledge 标记本人消息已读；feedback 非空时同时提交一次处理结果，重复提交返回冲突。
func (s *Service) Acknowledge(ctx context.Context, id uint64, feedback *string) error {
	if feedback != nil && strings.TrimSpace(*feedback) == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "请填写处理结果")
	}
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return err
	}
	return tx.Transaction(func(tx *gorm.DB) error {
		// SHARE 锁与发布/撤回互斥，但不同账号回执可并行。
		var n model.Notification
		if err := tx.Table(s.table("sys_notification")).Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ? AND status = ?", id, "published").First(&n).Error; err != nil {
			return dbError(err)
		}
		var r model.NotificationReceipt
		q := s.own(tx.Table(s.table("sys_notification_receipt")+" AS r"), ctx).Where("r.notification_id = ? AND r.deleted_at IS NULL", id)
		if err := q.Clauses(clause.Locking{Strength: "UPDATE"}).First(&r).Error; err != nil {
			return dbError(err)
		}
		updates := map[string]any{}
		now := time.Now()
		if r.ReadAt == nil {
			updates["read_at"] = now
		}
		if feedback != nil {
			if !n.RequireProcess {
				return errcode.Newf(errcode.ErrConflict, "该通知无需提交处理结果")
			}
			if r.ProcessedAt != nil {
				return errcode.Newf(errcode.ErrConflict, "该通知已处理")
			}
			updates["processed_at"], updates["feedback"] = now, strings.TrimSpace(*feedback)
		}
		if len(updates) == 0 {
			return nil
		}
		return dbError(tx.Table(s.table("sys_notification_receipt")).Where("id = ?", r.ID).Updates(updates).Error)
	})
}

// Targets 分页搜索可接收对象；租户只能搜索本租户账号。
func (s *Service) Targets(ctx context.Context, in TargetQuery) (*response.Page[Option], error) {
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return nil, err
	}
	isTenant := ctxkey.UserType(ctx) == model.UserTypeTenant
	if isTenant && in.Type != "tenant_users" {
		return nil, errcode.Newf(errcode.ErrForbiddenAction, "租户只能选择本租户账号")
	}
	var q *gorm.DB
	switch in.Type {
	case "tenants":
		q = tx.Model(&model.Tenant{}).Table(s.table("sys_tenant")).Select("id, name || ' (' || code || ')' AS label").Where("status = ? AND (expire_at IS NULL OR expire_at > ?)", model.StatusEnabled, time.Now())
	case "platform_users":
		q = tx.Model(&model.PlatformUser{}).Table(s.table("sys_platform_user")).Select("id, username || ' ' || nickname AS label").Where("status = ?", model.StatusEnabled)
	default:
		q = tx.Table(s.table("sys_tenant_user")+" AS u").Select("u.id, t.name || ' / ' || u.username || ' ' || u.nickname AS label").Joins("JOIN "+s.table("sys_tenant")+" AS t ON t.id = u.tenant_id").Where("u.deleted_at IS NULL AND t.deleted_at IS NULL AND u.status = ? AND t.status = ? AND (t.expire_at IS NULL OR t.expire_at > ?)", model.StatusEnabled, model.StatusEnabled, time.Now())
		if isTenant {
			q = q.Where("u.tenant_id = ?", ctxkey.TenantID(ctx))
		}
	}
	// 在子查询上搜索标签，使三类目标保持统一的分页查询协议。
	q = tx.Table("(?) AS targets", q)
	if kw := strings.TrimSpace(in.Keyword); kw != "" {
		q = q.Where("label ILIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, dbError(err)
	}
	var rows []Option
	if err := q.Order("id ASC").Limit(in.Limit()).Offset(in.Offset()).Scan(&rows).Error; err != nil {
		return nil, dbError(err)
	}
	return response.NewPage(rows, total), nil
}
