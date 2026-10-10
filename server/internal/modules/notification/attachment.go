package notification

import (
	"context"
	"gorm.io/gorm"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/modules/storage"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
)

// validateAttachments 校验并冻结附件快照；空列表直接返回，未配置存储时拒绝。
func (s *Service) validateAttachments(ctx context.Context, tx *gorm.DB, ids []uint64) ([]model.Attachment, error) {
	if len(ids) == 0 {
		return []model.Attachment{}, nil
	}
	if s.storage == nil {
		return nil, errcode.Newf(errcode.ErrServiceUnavailable, "附件存储尚未配置")
	}
	return s.storage.Attachments(ctx, tx, ids)
}

// DownloadAttachment 分别按发布治理权限或本人接收快照鉴权，再授权读取附件资源。
func (s *Service) DownloadAttachment(ctx context.Context, nid, rid uint64, management bool) (*storage.Download, error) {
	tx, err := s.inboxDB(ctx)
	if err != nil {
		return nil, err
	}
	var n model.Notification
	if management {
		err = s.managed(tx, ctx).First(&n, nid).Error
	} else {
		// 接收视角：先确认本人存在未撤回的接收快照，再读取通知本体。
		var count int64
		err = s.inboxQuery(tx, ctx).Where("n.id=?", nid).Count(&count).Error
		if err == nil && count == 0 {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		if err == nil {
			err = s.notifications(tx).First(&n, nid).Error
		}
	}
	if err != nil {
		return nil, dbError(err)
	}
	if ctxkey.UserID(ctx) == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}
	// 资源必须落在该通知冻结的附件快照内，防止借消息 ID 越权读取任意资源。
	found := false
	for _, a := range n.Attachments {
		if a.ResourceID == rid {
			found = true
			break
		}
	}
	if !found {
		return nil, errcode.New(errcode.ErrNotFound)
	}
	if s.storage == nil {
		return nil, errcode.New(errcode.ErrServiceUnavailable)
	}
	return s.storage.AttachmentDownload(ctx, rid)
}
