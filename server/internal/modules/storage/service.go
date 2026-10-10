// Package storage 管理平台 TOS 资源、字典套餐与租户配额。
package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"io"
	"pet-manager/server/internal/config"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/objectstore"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxCapacity 是单个存储空间容量上限（1 PiB），JSON 整数仍可精确表达。
const MaxCapacity int64 = 1 << 50

// Service 管理平台 TOS 资源台账、字典套餐与租户配额预占。
type Service struct {
	manager *db.Manager       // 平台库管理器，负责 schema 限定与事务
	store   objectstore.Store // 对象存储客户端，nil 表示 TOS 未启用
	cfg     config.TOSConfig  // 上传上限、桶名与 key 前缀等 TOS 配置
}

// NewService 构造 storage 服务；store 为 nil 时降级为仅维护台账的占位实现。
func NewService(m *db.Manager, store objectstore.Store, cfg config.TOSConfig) *Service {
	return &Service{manager: m, store: store, cfg: cfg}
}

// ListQuery 是存储资源与账户列表的通用查询参数。
type ListQuery struct {
	pagination.Query
	Keyword  string  `form:"keyword"`                                                   // 文件名或租户名称关键词
	TenantID *uint64 `form:"tenant_id"`                                                 // 平台按归属租户过滤，0 为平台
	Status   string  `form:"status" binding:"omitempty,oneof=uploading ready deleting"` // 资源状态
	Own      bool    `form:"own"`                                                       // 仅列出当前存储空间
}

// ResourceItem 在资源记录上附带归属租户名称。
type ResourceItem struct {
	model.StorageResource
	TenantName string `json:"tenant_name"`
}

// Summary 描述一个存储空间的套餐、配额上限与上传限制。
type Summary struct {
	model.StorageAccount
	QuotaBytes     int64 `json:"quota_bytes"`      // 基础容量 + 扩容容量
	Unlimited      bool  `json:"unlimited"`        // 平台公共存储不限额
	Enabled        bool  `json:"enabled"`          // TOS 是否启用
	MaxUploadBytes int64 `json:"max_upload_bytes"` // 单文件上传上限
}

// AccountItem 是平台视图下的租户存储账户，附租户名称与编码。
type AccountItem struct {
	Summary
	TenantName string `json:"tenant_name"`
	TenantCode string `json:"tenant_code"`
}

// AllocationInput 是平台分配套餐（assign）或追加容量（expand）的入参。
type AllocationInput struct {
	Action   string `json:"action" binding:"required,oneof=assign expand"`
	PlanID   uint64 `json:"plan_id"`   // assign 时的目标套餐
	AddBytes int64  `json:"add_bytes"` // expand 时的追加字节数
	Remark   string `json:"remark" binding:"max=255"`
}

// Download 是对外返回的临时下载地址，不含任何存储凭证。
type Download struct {
	URL string `json:"url"`
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

// requestDB 按调用方身份返回只读数据库句柄：租户用请求事务，平台用平台库。
func (s *Service) requestDB(ctx context.Context) (*gorm.DB, error) {
	if ctxkey.UserType(ctx) == model.UserTypeTenant && ctxkey.TenantID(ctx) > 0 {
		return ctxkey.MustTenantDB(ctx)
	}
	if ctxkey.UserType(ctx) != model.UserTypePlatform || ctxkey.TenantID(ctx) != 0 {
		return nil, errcode.New(errcode.ErrForbiddenAction)
	}
	return s.manager.Platform().WithContext(ctx), nil
}

// 对象操作跨网络：先独立提交资源台账与容量预占，再调用 TOS，避免外层请求回滚丢失清理记录。
// 租户独立写事务同样必须 SET LOCAL search_path，绝不借用 Platform()。
func (s *Service) write(ctx context.Context, fn func(*gorm.DB) error) error {
	if ctxkey.UserType(ctx) == model.UserTypeTenant {
		if ctxkey.TenantID(ctx) == 0 || ctxkey.TenantSchema(ctx) == "" {
			return errcode.New(errcode.ErrForbiddenAction)
		}
		return s.manager.TenantTx(ctx, ctxkey.TenantSchema(ctx), fn)
	}
	if ctxkey.UserType(ctx) != model.UserTypePlatform || ctxkey.TenantID(ctx) != 0 {
		return errcode.New(errcode.ErrForbiddenAction)
	}
	return s.manager.Platform().WithContext(ctx).Transaction(fn)
}

// resources 返回平台 schema 下的存储资源查询。
func (s *Service) resources(tx *gorm.DB) *gorm.DB {
	return tx.Model(&model.StorageResource{}).Table(s.table("sys_storage_resource"))
}

// accounts 返回平台 schema 下的租户存储账户查询。
func (s *Service) accounts(tx *gorm.DB) *gorm.DB {
	return tx.Model(&model.StorageAccount{}).Table(s.table("sys_storage_account"))
}

// plans 返回平台字典 storage.plan 中启用且容量有效的套餐。
func (s *Service) plans(tx *gorm.DB) *gorm.DB {
	return tx.Table(s.table("sys_dict_item")+" AS i").Joins("JOIN "+s.table("sys_dict")+" AS d ON d.id=i.dict_id").Where("i.deleted_at IS NULL AND d.deleted_at IS NULL AND d.code=? AND d.status=? AND i.status=? AND i.storage_bytes>0 AND i.storage_bytes<=?", "storage.plan", "enabled", "enabled", MaxCapacity)
}

// Plans 列出平台字典中启用的存储套餐。
func (s *Service) Plans(ctx context.Context) ([]model.DictItem, error) {
	tx, err := s.requestDB(ctx)
	if err != nil {
		return nil, err
	}
	rows := []model.DictItem{}
	err = s.plans(tx).Select("i.*").Order("i.sort ASC,i.id ASC").Scan(&rows).Error
	return rows, dbError(err)
}

// lockAccount 先创建缺失账户（冲突忽略），再锁行；并发上传共享同一配额锁。
func (s *Service) lockAccount(tx *gorm.DB, tid uint64) (*model.StorageAccount, error) {
	var a model.StorageAccount
	err := s.accounts(tx).Where("tenant_id=?", tid).Clauses(clause.Locking{Strength: "UPDATE"}).First(&a).Error
	if err == nil {
		return &a, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, dbError(err)
	}
	a.TenantID = tid
	if tid > 0 {
		var tenant model.Tenant
		if err := tx.Table(s.table("sys_tenant")).Clauses(clause.Locking{Strength: "SHARE"}).First(&tenant, tid).Error; err != nil {
			return nil, dbError(err)
		}
		var plan model.DictItem
		if err := s.plans(tx).Select("i.*").Order("i.sort ASC,i.id ASC").Limit(1).Scan(&plan).Error; err != nil {
			return nil, dbError(err)
		}
		if plan.ID == 0 {
			return nil, errcode.Newf(errcode.ErrConflict, "请先在平台字典 storage.plan 配置启用的存储套餐")
		}
		a.PlanID, a.PlanName, a.BaseBytes = plan.ID, plan.Label, plan.StorageBytes
	} else {
		a.PlanName = "平台公共存储"
	}
	// 冲突忽略：并发首个请求创建，其余落到随后的加锁读取。
	if err := s.accounts(tx).Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error; err != nil {
		return nil, dbError(err)
	}
	a = model.StorageAccount{}
	if err := s.accounts(tx).Where("tenant_id=?", tid).Clauses(clause.Locking{Strength: "UPDATE"}).First(&a).Error; err != nil {
		return nil, dbError(err)
	}
	return &a, nil
}

// summary 组装对外返回的存储空间摘要。
func (s *Service) summary(a model.StorageAccount) Summary {
	return Summary{StorageAccount: a, QuotaBytes: a.BaseBytes + a.ExtraBytes, Unlimited: a.TenantID == 0, Enabled: s.store != nil, MaxUploadBytes: s.cfg.MaxUploadBytes}
}

// Summary 返回当前存储空间的套餐、占用与上传限制。
func (s *Service) Summary(ctx context.Context) (*Summary, error) {
	var a *model.StorageAccount
	err := s.write(ctx, func(tx *gorm.DB) error { var err error; a, err = s.lockAccount(tx, ctxkey.TenantID(ctx)); return err })
	if err != nil {
		return nil, err
	}
	v := s.summary(*a)
	return &v, nil
}

// List 分页查询存储资源：租户仅见本租户，平台可按 tenant_id/own 过滤。
func (s *Service) List(ctx context.Context, in ListQuery) (*response.Page[ResourceItem], error) {
	tx, err := s.requestDB(ctx)
	if err != nil {
		return nil, err
	}
	q := tx.Table(s.table("sys_storage_resource") + " AS r").Joins("LEFT JOIN " + s.table("sys_tenant") + " AS t ON t.id=r.tenant_id").Where("r.deleted_at IS NULL")
	if ctxkey.UserType(ctx) == model.UserTypeTenant || in.Own {
		q = q.Where("r.tenant_id=?", ctxkey.TenantID(ctx))
	} else if in.TenantID != nil {
		q = q.Where("r.tenant_id=?", *in.TenantID)
	}
	if in.Keyword != "" {
		q = q.Where("r.name ILIKE ?", "%"+strings.TrimSpace(in.Keyword)+"%")
	}
	if in.Status != "" {
		q = q.Where("r.status=?", in.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, dbError(err)
	}
	rows := []ResourceItem{}
	err = q.Select("r.*, CASE WHEN r.tenant_id=0 THEN '平台' ELSE COALESCE(t.name,'已销毁租户') END AS tenant_name").Order("r.id DESC").Limit(in.Limit()).Offset(in.Offset()).Scan(&rows).Error
	return response.NewPage(rows, total), dbError(err)
}

// Upload 在独立事务内预占配额并写台账，随后上传 TOS，成功后再落 ready 状态。
// 文件名会规范化，空文件与超限文件直接拒绝；上传失败转 deleting 并由后台清理重试。
func (s *Service) Upload(ctx context.Context, name, ct string, size int64, body io.Reader) (*model.StorageResource, error) {
	if s.store == nil {
		return nil, errcode.Newf(errcode.ErrServiceUnavailable, "平台 TOS 存储未启用")
	}
	// 统一分隔符并只取基名，防止路径穿越；再校验合法性与长度。
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	name = name[strings.LastIndex(name, "/")+1:]
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 || strings.ContainsAny(name, "\r\n\x00") || size <= 0 {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "文件名或文件大小无效，不支持空文件")
	}
	if size > s.cfg.MaxUploadBytes {
		return nil, errcode.Newf(errcode.ErrRequestTooLarge, "文件超过单文件上传限制")
	}
	if len(ct) > 128 {
		ct = "application/octet-stream"
	}
	r := model.StorageResource{TenantID: ctxkey.TenantID(ctx), Name: name, SizeBytes: size, ContentType: ct, Bucket: s.cfg.Bucket, ObjectKey: strings.Trim(s.cfg.KeyPrefix, "/") + fmt.Sprintf("/storage/%d/%s", ctxkey.TenantID(ctx), rand.Text()), Status: "uploading", UploaderID: ctxkey.UserID(ctx), UploaderName: ctxkey.Username(ctx)}
	r.ObjectKey = strings.TrimLeft(r.ObjectKey, "/")
	// 先校验租户状态并预占配额，任一不满足则整体回滚。
	err := s.write(ctx, func(tx *gorm.DB) error {
		if r.TenantID > 0 {
			var tenant model.Tenant
			if err := tx.Table(s.table("sys_tenant")).Clauses(clause.Locking{Strength: "SHARE"}).First(&tenant, r.TenantID).Error; err != nil {
				return dbError(err)
			}
			if tenant.Status != model.StatusEnabled {
				return errcode.New(errcode.ErrTenantDisabled)
			}
		}
		a, err := s.lockAccount(tx, r.TenantID)
		if err != nil {
			return err
		}
		if r.TenantID > 0 && (size > a.BaseBytes+a.ExtraBytes || a.UsedBytes > a.BaseBytes+a.ExtraBytes-size) {
			return errcode.Newf(errcode.ErrConflict, "存储空间不足，请联系平台升级套餐或扩容")
		}
		if err := s.resources(tx).Create(&r).Error; err != nil {
			return dbError(err)
		}
		return dbError(s.accounts(tx).Where("id=?", a.ID).Update("used_bytes", gorm.Expr("used_bytes + ?", size)).Error)
	})
	if err != nil {
		return nil, err
	}
	// 台账与预占已提交，再执行可能较慢的 TOS 上传。
	if err = s.store.Put(ctx, r.Bucket, r.ObjectKey, r.Name, r.ContentType, r.SizeBytes, io.LimitReader(body, size)); err != nil {
		// 上传失败：转 deleting 并尽力清理，预占容量由清理流程释放。
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_ = s.write(cleanupCtx, func(tx *gorm.DB) error {
			return dbError(s.resources(tx).Where("id=? AND status=?", r.ID, "uploading").Update("status", "deleting").Error)
		})
		_ = s.cleanOne(cleanupCtx, r)
		return nil, errcode.Newf(errcode.ErrServiceUnavailable, "文件上传失败，请稍后重试")
	}
	// 上传完成再复核租户是否仍存活，避免为已销毁租户留下就绪文件。
	finalReady := true
	err = s.write(ctx, func(tx *gorm.DB) error {
		status := "ready"
		if r.TenantID > 0 {
			var tenant model.Tenant
			err := tx.Table(s.table("sys_tenant")).Clauses(clause.Locking{Strength: "SHARE"}).First(&tenant, r.TenantID).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return dbError(err)
			}
			if errors.Is(err, gorm.ErrRecordNotFound) || tenant.Status == model.StatusDeleted {
				status = "deleting"
				finalReady = false
			}
		}
		res := s.resources(tx).Where("id=? AND status=?", r.ID, "uploading").Update("status", status)
		if res.Error != nil {
			return dbError(res.Error)
		}
		if res.RowsAffected != 1 {
			return errcode.New(errcode.ErrConflict)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !finalReady {
		return nil, errcode.Newf(errcode.ErrConflict, "租户已销毁，上传文件将由后台清理")
	}
	r.Status = "ready"
	return &r, nil
}

// owned 对租户调用追加归属过滤，平台调用不加限制。
func (s *Service) owned(q *gorm.DB, ctx context.Context) *gorm.DB {
	if ctxkey.UserType(ctx) == model.UserTypeTenant {
		return q.Where("tenant_id=?", ctxkey.TenantID(ctx))
	}
	return q
}

// Delete 删除资源：先将台账置为 deleting，再尝试清理 TOS 对象，失败交由后台重试。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	if s.store == nil {
		return errcode.New(errcode.ErrServiceUnavailable)
	}
	var r model.StorageResource
	err := s.write(ctx, func(tx *gorm.DB) error {
		if err := s.owned(s.resources(tx), ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&r, id).Error; err != nil {
			return dbError(err)
		}
		if r.Status == "uploading" {
			return errcode.Newf(errcode.ErrConflict, "文件正在上传，请稍后删除")
		}
		return dbError(s.resources(tx).Where("id=?", r.ID).Update("status", "deleting").Error)
	})
	if err != nil {
		return err
	}
	if err := s.cleanOne(ctx, r); err != nil {
		return errcode.Newf(errcode.ErrServiceUnavailable, "已标记待清理，TOS 删除暂未完成，系统会自动重试")
	}
	return nil
}

// cleanOne 先删除对象，再幂等释放预占容量。对象清理失败时台账与容量均保留。
func (s *Service) cleanOne(ctx context.Context, r model.StorageResource) error {
	if err := s.store.Delete(ctx, r.Bucket, r.ObjectKey); err != nil {
		return err
	}
	return s.write(ctx, func(tx *gorm.DB) error {
		a, err := s.lockAccount(tx, r.TenantID)
		if err != nil {
			return err
		}
		var row model.StorageResource
		err = s.resources(tx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, r.ID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return dbError(err)
		}
		if row.Status != "deleting" {
			return errcode.New(errcode.ErrConflict)
		}
		if err := s.resources(tx).Delete(&row).Error; err != nil {
			return dbError(err)
		}
		if err := s.accounts(tx).Where("id=?", a.ID).Update("used_bytes", gorm.Expr("GREATEST(used_bytes - ?, 0)", row.SizeBytes)).Error; err != nil {
			return dbError(err)
		}
		if r.TenantID > 0 {
			var tenants, pending int64
			if err := tx.Model(&model.Tenant{}).Table(s.table("sys_tenant")).Where("id=?", r.TenantID).Count(&tenants).Error; err != nil {
				return dbError(err)
			}
			if tenants == 0 {
				if err := s.resources(tx).Where("tenant_id=?", r.TenantID).Count(&pending).Error; err != nil {
					return dbError(err)
				}
				if pending == 0 {
					return dbError(s.accounts(tx).Unscoped().Where("id=?", a.ID).Delete(&model.StorageAccount{}).Error)
				}
			}
		}
		return nil
	})
}

// Cleanup 仅平台后台任务调用。半小时未完成的上传与待清理对象统一重试。
func (s *Service) Cleanup(ctx context.Context) (string, error) {
	if s.store == nil {
		return "平台 TOS 未启用", nil
	}
	ctx = ctxkey.WithUser(ctx, 0, "存储清理任务", model.UserTypePlatform)
	rows := []model.StorageResource{}
	if err := s.resources(s.manager.Platform().WithContext(ctx)).Where("status=? OR (status=? AND updated_at<?)", "deleting", "uploading", time.Now().Add(-30*time.Minute)).Order("id ASC").Limit(100).Find(&rows).Error; err != nil {
		return "", dbError(err)
	}
	count, failed := 0, 0
	for _, r := range rows {
		if r.Status == "uploading" {
			// 仅认领超过 30 分钟仍处于 uploading 的记录，条件更新保证同一记录只被一个执行者处理。
			claimed := false
			err := s.write(ctx, func(tx *gorm.DB) error {
				res := s.resources(tx).Where("id=? AND status=? AND updated_at<?", r.ID, "uploading", time.Now().Add(-30*time.Minute)).Update("status", "deleting")
				claimed = res.RowsAffected == 1
				return dbError(res.Error)
			})
			if err != nil {
				failed++
				continue
			}
			if !claimed {
				continue
			}
		}
		if err := s.cleanOne(ctx, r); err != nil {
			failed++
		} else {
			count++
		}
	}
	if failed > 0 {
		return fmt.Sprintf("已清理 %d 个，待重试 %d 个", count, failed), errcode.New(errcode.ErrServiceUnavailable)
	}
	return fmt.Sprintf("已清理 %d 个资源", count), nil
}

// Download 为本人或本租户的就绪资源签发临时下载地址。
func (s *Service) Download(ctx context.Context, id uint64) (*Download, error) {
	tx, err := s.requestDB(ctx)
	if err != nil {
		return nil, err
	}
	var r model.StorageResource
	if err := s.owned(s.resources(tx), ctx).Where("status=?", "ready").First(&r, id).Error; err != nil {
		return nil, dbError(err)
	}
	return s.sign(ctx, r)
}

// sign 调用对象存储生成预签名 URL，凭证不下发到业务响应。
func (s *Service) sign(ctx context.Context, r model.StorageResource) (*Download, error) {
	if s.store == nil {
		return nil, errcode.New(errcode.ErrServiceUnavailable)
	}
	u, err := s.store.Sign(ctx, r.Bucket, r.ObjectKey, r.Name)
	if err != nil {
		return nil, errcode.Newf(errcode.ErrServiceUnavailable, "暂时无法下载文件")
	}
	return &Download{URL: u}, nil
}

// AttachmentDownload 只能在通知模块校验发送方/接收快照后调用，不挂独立路由。
func (s *Service) AttachmentDownload(ctx context.Context, id uint64) (*Download, error) {
	tx, err := s.requestDB(ctx)
	if err != nil {
		return nil, err
	}
	var r model.StorageResource
	if err := s.resources(tx).Where("status=?", "ready").First(&r, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.Newf(errcode.ErrNotFound, "附件已删除或正在清理，无法下载")
		}
		return nil, dbError(err)
	}
	return s.sign(ctx, r)
}

// Attachments 附件只可引用当前平台/租户自己拥有的就绪资源，去重并冻结显示信息。
func (s *Service) Attachments(ctx context.Context, tx *gorm.DB, ids []uint64) ([]model.Attachment, error) {
	rows := []model.Attachment{}
	if len(ids) > 10 {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "每条消息最多 10 个附件")
	}
	seen := map[uint64]bool{}
	for _, id := range ids {
		if id == 0 {
			return nil, errcode.New(errcode.ErrInvalidParam)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		var r model.StorageResource
		if err := s.resources(tx).Where("tenant_id=? AND status=?", ctxkey.TenantID(ctx), "ready").Clauses(clause.Locking{Strength: "SHARE"}).First(&r, id).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, dbError(err)
			}
			return nil, errcode.Newf(errcode.ErrInvalidParam, "附件不存在、已删除或不属于当前存储空间")
		}
		rows = append(rows, model.Attachment{ResourceID: r.ID, Name: r.Name, SizeBytes: r.SizeBytes, ContentType: r.ContentType})
	}
	return rows, nil
}

// Accounts 平台分页列出各租户存储账户，缺失账户按默认套餐补建。
func (s *Service) Accounts(ctx context.Context, in ListQuery) (*response.Page[AccountItem], error) {
	if ctxkey.UserType(ctx) != model.UserTypePlatform {
		return nil, errcode.New(errcode.ErrForbiddenAction)
	}
	tx, err := s.requestDB(ctx)
	if err != nil {
		return nil, err
	}
	q := tx.Model(&model.Tenant{}).Table(s.table("sys_tenant"))
	if in.Keyword != "" {
		q = q.Where("name ILIKE ? OR code ILIKE ?", "%"+in.Keyword+"%", "%"+in.Keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, dbError(err)
	}
	tenants := []model.Tenant{}
	if err := q.Order("id DESC").Limit(in.Limit()).Offset(in.Offset()).Find(&tenants).Error; err != nil {
		return nil, dbError(err)
	}
	rows := []AccountItem{}
	for _, t := range tenants {
		var a *model.StorageAccount
		if err := s.write(ctx, func(tx *gorm.DB) error { var err error; a, err = s.lockAccount(tx, t.ID); return err }); err != nil {
			return nil, err
		}
		rows = append(rows, AccountItem{Summary: s.summary(*a), TenantName: t.Name, TenantCode: t.Code})
	}
	return response.NewPage(rows, total), nil
}

// Allocate 平台为租户分配套餐或追加容量，并写入配额变更记录。
func (s *Service) Allocate(ctx context.Context, tid uint64, in AllocationInput) (*Summary, error) {
	if ctxkey.UserType(ctx) != model.UserTypePlatform || tid == 0 {
		return nil, errcode.New(errcode.ErrForbiddenAction)
	}
	var result Summary
	err := s.write(ctx, func(tx *gorm.DB) error {
		var tenant model.Tenant
		if err := tx.Table(s.table("sys_tenant")).Clauses(clause.Locking{Strength: "SHARE"}).First(&tenant, tid).Error; err != nil {
			return dbError(err)
		}
		a, err := s.lockAccount(tx, tid)
		if err != nil {
			return err
		}
		before := a.BaseBytes + a.ExtraBytes
		switch in.Action {
		case "assign":
			var plan model.DictItem
			if err := s.plans(tx).Select("i.*").Where("i.id=?", in.PlanID).Scan(&plan).Error; err != nil {
				return dbError(err)
			}
			if plan.ID == 0 {
				return errcode.Newf(errcode.ErrInvalidParam, "请选择启用且容量有效的存储套餐")
			}
			if plan.StorageBytes+a.ExtraBytes > MaxCapacity {
				return errcode.New(errcode.ErrInvalidParam)
			}
			if plan.StorageBytes+a.ExtraBytes < a.UsedBytes {
				return errcode.Newf(errcode.ErrConflict, "调整后的配额不能小于已占用容量，请先清理文件")
			}
			a.PlanID, a.PlanName, a.BaseBytes = plan.ID, plan.Label, plan.StorageBytes
		case "expand":
			if in.AddBytes <= 0 || in.AddBytes > MaxCapacity-before {
				return errcode.Newf(errcode.ErrInvalidParam, "扩容字节数必须大于 0，且总容量不得超过 1 PiB")
			}
			a.ExtraBytes += in.AddBytes
		default:
			return errcode.New(errcode.ErrInvalidParam)
		}
		if err := s.accounts(tx).Where("id=?", a.ID).Save(a).Error; err != nil {
			return dbError(err)
		}
		change := model.StorageChange{TenantID: tid, Action: in.Action, PlanName: a.PlanName, BeforeBytes: before, AfterBytes: a.BaseBytes + a.ExtraBytes, OperatorID: ctxkey.UserID(ctx), OperatorName: ctxkey.Username(ctx), Remark: in.Remark}
		if err := tx.Table(s.table("sys_storage_change")).Create(&change).Error; err != nil {
			return dbError(err)
		}
		result = s.summary(*a)
		return nil
	})
	return &result, err
}

// Changes 分页查询指定租户的存储配额变更记录。
func (s *Service) Changes(ctx context.Context, tid uint64, in ListQuery) (*response.Page[model.StorageChange], error) {
	if ctxkey.UserType(ctx) != model.UserTypePlatform {
		return nil, errcode.New(errcode.ErrForbiddenAction)
	}
	tx, err := s.requestDB(ctx)
	if err != nil {
		return nil, err
	}
	q := tx.Model(&model.StorageChange{}).Table(s.table("sys_storage_change")).Where("tenant_id=?", tid)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, dbError(err)
	}
	rows := []model.StorageChange{}
	err = q.Order("id DESC").Limit(in.Limit()).Offset(in.Offset()).Find(&rows).Error
	return response.NewPage(rows, total), dbError(err)
}
