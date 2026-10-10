// 本文件承载租户侧参数配置（t_config）的增删改查。
//
// 与平台侧共用 Item/Input/ListQuery 契约，区别只在取数入口：
// 平台侧走 manager.Platform()，租户侧只能走请求上下文里的租户事务句柄
// （TenantScope 中间件已把 search_path 设为 "tenant_x, public"），
// 从而天然按 schema 隔离，租户之间互不可见。

package config

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/ctxkey"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/response"
)

// ListTenant 分页查询当前租户的参数配置。
func (s *Service) ListTenant(ctx context.Context, in ListQuery) (*response.Page[Item], error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	q := tx.Model(&model.TenantConfig{})

	if kw := strings.TrimSpace(in.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("config_key ILIKE ? OR name ILIKE ?", like, like)
	}
	if g := strings.TrimSpace(in.GroupKey); g != "" {
		q = q.Where("group_key = ?", g)
	}
	if st := strings.TrimSpace(in.Status); st != "" {
		q = q.Where("status = ?", st)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	var rows []model.TenantConfig
	if err := q.Order("id desc").Limit(in.Limit()).Offset(in.Offset()).Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	list := make([]Item, 0, len(rows))
	for _, r := range rows {
		list = append(list, toTenantItem(r))
	}
	return response.NewPage(list, total), nil
}

// CreateTenant 在当前租户下新增参数配置。
func (s *Service) CreateTenant(ctx context.Context, in Input) (*Item, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	in.ConfigKey = strings.TrimSpace(in.ConfigKey)
	if err := ensureTenantKeyFree(tx, in.ConfigKey, 0); err != nil {
		return nil, err
	}

	c := model.TenantConfig{
		ConfigKey: in.ConfigKey,
		Name:      in.Name,
		Value:     in.Value,
		GroupKey:  strings.TrimSpace(in.GroupKey),
		Type:      in.Type,
		Remark:    in.Remark,
		Status:    normalizeStatus(in.Status),
	}
	if err := tx.Create(&c).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	item := toTenantItem(c)
	return &item, nil
}

// UpdateTenant 编辑当前租户的参数配置。参数键允许修改，但需保持存活唯一。
func (s *Service) UpdateTenant(ctx context.Context, id uint64, in Input) (*Item, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	var c model.TenantConfig
	if err := tx.First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	in.ConfigKey = strings.TrimSpace(in.ConfigKey)
	if err := ensureTenantKeyFree(tx, in.ConfigKey, id); err != nil {
		return nil, err
	}

	c.ConfigKey = in.ConfigKey
	c.Name = in.Name
	c.Value = in.Value
	c.GroupKey = strings.TrimSpace(in.GroupKey)
	c.Type = in.Type
	c.Remark = in.Remark
	c.Status = normalizeStatus(in.Status)
	if err := tx.Save(&c).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	item := toTenantItem(c)
	return &item, nil
}

// DeleteTenant 软删当前租户的参数配置。
func (s *Service) DeleteTenant(ctx context.Context, id uint64) error {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return err
	}
	res := tx.Delete(&model.TenantConfig{}, id)
	if res.Error != nil {
		return errcode.Wrap(errcode.ErrDatabase, res.Error)
	}
	if res.RowsAffected == 0 {
		return errcode.New(errcode.ErrNotFound)
	}
	return nil
}

// ensureTenantKeyFree 校验租户参数键在当前 schema 存活唯一。
func ensureTenantKeyFree(tx *gorm.DB, key string, selfID uint64) error {
	if key == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "参数键不能为空")
	}
	q := tx.Model(&model.TenantConfig{}).Where("config_key = ?", key)
	if selfID > 0 {
		q = q.Where("id <> ?", selfID)
	}
	var dup int64
	if err := q.Count(&dup).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if dup > 0 {
		return errcode.Newf(errcode.ErrConflict, "参数键已存在")
	}
	return nil
}

// toTenantItem 把租户存储模型转换为对外条目（与平台侧共用 Item 契约）。
func toTenantItem(c model.TenantConfig) Item {
	return Item{
		ID:        c.ID,
		ConfigKey: c.ConfigKey,
		Name:      c.Name,
		Value:     c.Value,
		GroupKey:  c.GroupKey,
		Type:      c.Type,
		Remark:    c.Remark,
		Status:    c.Status,
		CreatedAt: c.CreatedAt,
	}
}
