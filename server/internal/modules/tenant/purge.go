package tenant

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/infra/session"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"time"
)

// DestroyInput 是立即销毁租户的确认入参。
type DestroyInput struct {
	ConfirmCode string `json:"confirm_code" binding:"required,max=64"` // 必须精确匹配的租户编码
}

// DestroyResult 是销毁结果。
type DestroyResult struct {
	PendingResources int64 `json:"pending_resources"` // 转入后台清理队列的存储对象数
}

// Destroy 仅平台立即销毁单个租户，编码必须精确匹配，不执行全局到期任务。
func (s *Service) Destroy(ctx context.Context, id uint64, in DestroyInput) (*DestroyResult, error) {
	if ctxkey.UserType(ctx) != model.UserTypePlatform {
		return nil, errcode.New(errcode.ErrForbiddenAction)
	}
	return s.purgeOne(ctx, id, &in.ConfirmCode)
}

// table 拼出平台库中带 schema 限定的表名，供裸 SQL 引用平台表。
func (s *Service) table(name string) string {
	return fmt.Sprintf("%q.%q", s.manager.PlatformSchema(), name)
}

// purgeOne 用同一 PostgreSQL 事务完成 schema/业务记录销毁；定时与立即销毁复用行锁。
// confirm=nil 时必须重新确认已经到期，防止定时任务使用过期扫描结果销毁已恢复租户。
func (s *Service) purgeOne(ctx context.Context, id uint64, confirm *string) (*DestroyResult, error) {
	result := &DestroyResult{}
	cacheMarked := false
	err := s.manager.Platform().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var t model.Tenant
		err := tx.Table(s.table("sys_tenant")).Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errcode.New(errcode.ErrTenantNotFound)
		}
		if err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if confirm != nil {
			if *confirm != t.Code {
				return errcode.Newf(errcode.ErrInvalidParam, "租户编码不匹配，未执行销毁")
			}
		} else if t.Status != model.StatusDeleted || t.PurgeAt == nil || t.PurgeAt.After(time.Now()) {
			return errcode.Newf(errcode.ErrConflict, "租户未到销毁时间或已恢复")
		}
		if err := db.ValidateSchema(t.SchemaName); err != nil {
			return err
		}
		if t.SchemaName != db.SchemaForTenant(t.ID) {
			return errcode.Newf(errcode.ErrTenantSchemaBad, "租户数据空间与租户 ID 不匹配，拒绝销毁")
		}
		var users []model.TenantUser
		if err := tx.Table(s.table("sys_tenant_user")).Unscoped().Where("tenant_id=?", id).Find(&users).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		for _, u := range users {
			if err := s.sessions.RevokeUserSessions(ctx, model.UserTypeTenant, u.ID); err != nil {
				return errcode.Newf(errcode.ErrServiceUnavailable, "无法注销租户会话，未执行销毁，请稍后重试")
			}
		}
		if err := s.sessions.InvalidateTenantPermissions(ctx, id); err != nil {
			return errcode.Newf(errcode.ErrServiceUnavailable, "无法清理租户权限缓存，未执行销毁")
		}
		// 销毁期间缓存明确禁止访问；若提交后失效缓存失败，此 deleted 快照仍能拦截旧凭证。
		info := session.TenantInfo{ID: t.ID, Code: t.Code, Schema: t.SchemaName, Name: t.Name, Status: model.StatusDeleted, ExpireAt: t.ExpireAt}
		if err := s.sessions.CacheTenant(ctx, info); err != nil {
			return errcode.Newf(errcode.ErrServiceUnavailable, "无法更新租户状态缓存，未执行销毁")
		}
		cacheMarked = true
		if err := tx.Exec("DROP SCHEMA IF EXISTS " + t.SchemaName + " CASCADE").Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		// 正在上传的资源等上传结束或超时回收，避免对象删除后在途 PUT 再写回来。
		if err := tx.Table(s.table("sys_storage_resource")).Model(&model.StorageResource{}).Where("tenant_id=? AND status<>?", id, "uploading").Update("status", "deleting").Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if err := tx.Table(s.table("sys_storage_resource")).Model(&model.StorageResource{}).Where("tenant_id=?", id).Count(&result.PendingResources).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		// 消息、回执、账号、套餐变更记录均在平台表，需要按归属一起清理。
		if err := tx.Table(s.table("sys_notification_receipt")).Unscoped().Where("tenant_id=? AND user_type=?", id, model.UserTypeTenant).Delete(&model.NotificationReceipt{}).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if err := tx.Table(s.table("sys_notification")).Unscoped().Where("sender_type=? AND sender_tenant_id=?", model.UserTypeTenant, id).Delete(&model.Notification{}).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if err := tx.Table(s.table("sys_tenant_user")).Unscoped().Where("tenant_id=?", id).Delete(&model.TenantUser{}).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if err := tx.Table(s.table("sys_storage_change")).Unscoped().Where("tenant_id=?", id).Delete(&model.StorageChange{}).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		if result.PendingResources == 0 {
			if err := tx.Table(s.table("sys_storage_account")).Unscoped().Where("tenant_id=?", id).Delete(&model.StorageAccount{}).Error; err != nil {
				return errcode.Wrap(errcode.ErrDatabase, err)
			}
		}
		if err := tx.Table(s.table("sys_tenant")).Unscoped().Delete(&t).Error; err != nil {
			return errcode.Wrap(errcode.ErrDatabase, err)
		}
		return nil
	})
	if cacheMarked {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		s.invalidate(cleanupCtx, id)
	}
	if err != nil {
		return nil, err
	}
	s.log.Info("租户已物理销毁", zap.Uint64("tenant_id", id), zap.Int64("pending_storage_resources", result.PendingResources))
	return result, nil
}
