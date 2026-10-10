// Package config 承载参数配置（平台侧 sys_config 与租户侧 t_config）的增删改查。
//
// 定位：不改代码即可调整的参数（站点名称、开关、租户业务参数等），供后台治理与后续业务读取。
// 平台侧与租户侧共用 Item/Input 契约，区别只在取数入口（manager.Platform() vs
// 请求上下文里的租户事务句柄），见 tenant_service.go。参数配置变更不涉及权限字典，
// 因此无需失效权限缓存。
package config

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
	"pet-manager/server/internal/pkg/errcode"
	"pet-manager/server/internal/pkg/pagination"
	"pet-manager/server/internal/pkg/response"
)

// Item 是对外的参数条目。
type Item struct {
	ID        uint64    `json:"id"`         // 主键
	ConfigKey string    `json:"config_key"` // 参数键（存活唯一）
	Name      string    `json:"name"`       // 参数名称
	Value     string    `json:"value"`      // 参数值（按 Type 解释）
	GroupKey  string    `json:"group_key"`  // 分组键（如 audit）
	Type      string    `json:"type"`       // 值类型：string/number/bool/json
	Remark    string    `json:"remark"`     // 备注
	Status    string    `json:"status"`     // 状态：enabled/disabled
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// Input 是新增/编辑参数的入参。
type Input struct {
	ConfigKey string `json:"config_key" binding:"required,max=128"`                 // 参数键
	Name      string `json:"name" binding:"required,max=64"`                        // 参数名称
	Value     string `json:"value"`                                                 // 参数值
	GroupKey  string `json:"group_key" binding:"max=64"`                            // 分组键
	Type      string `json:"type" binding:"required,oneof=string number bool json"` // 值类型
	Remark    string `json:"remark" binding:"max=255"`                              // 备注
	Status    string `json:"status"`                                                // 状态，非 disabled 一律按 enabled
}

// ListQuery 是列表查询参数。
type ListQuery struct {
	pagination.Query
	Keyword  string `form:"keyword"`   // 键名/名称关键字
	GroupKey string `form:"group_key"` // 按分组筛选
	Status   string `form:"status"`    // 按状态筛选
}

// Service 承载平台侧参数配置（sys_config）的读写；租户侧见 tenant_service.go。
type Service struct {
	manager *db.Manager
}

// NewService 构造参数配置服务。
func NewService(manager *db.Manager) *Service {
	return &Service{manager: manager}
}

// List 分页查询参数配置。
func (s *Service) List(ctx context.Context, in ListQuery) (*response.Page[Item], error) {
	q := s.manager.Platform().WithContext(ctx).Model(&model.Config{})

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

	var rows []model.Config
	if err := q.Order("id desc").Limit(in.Limit()).Offset(in.Offset()).Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	list := make([]Item, 0, len(rows))
	for _, r := range rows {
		list = append(list, toItem(r))
	}
	return response.NewPage(list, total), nil
}

// Create 新增参数配置。
func (s *Service) Create(ctx context.Context, in Input) (*Item, error) {
	in.ConfigKey = strings.TrimSpace(in.ConfigKey)
	if err := s.ensureKeyFree(ctx, in.ConfigKey, 0); err != nil {
		return nil, err
	}

	c := model.Config{
		ConfigKey: in.ConfigKey,
		Name:      in.Name,
		Value:     in.Value,
		GroupKey:  strings.TrimSpace(in.GroupKey),
		Type:      in.Type,
		Remark:    in.Remark,
		Status:    normalizeStatus(in.Status),
	}
	if err := s.manager.Platform().WithContext(ctx).Create(&c).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	item := toItem(c)
	return &item, nil
}

// Update 编辑参数配置。参数键允许修改，但需保持存活唯一。
func (s *Service) Update(ctx context.Context, id uint64, in Input) (*Item, error) {
	tx := s.manager.Platform().WithContext(ctx)
	var c model.Config
	if err := tx.First(&c, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	in.ConfigKey = strings.TrimSpace(in.ConfigKey)
	if err := s.ensureKeyFree(ctx, in.ConfigKey, id); err != nil {
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
	item := toItem(c)
	return &item, nil
}

// Delete 软删参数配置。
func (s *Service) Delete(ctx context.Context, id uint64) error {
	res := s.manager.Platform().WithContext(ctx).Delete(&model.Config{}, id)
	if res.Error != nil {
		return errcode.Wrap(errcode.ErrDatabase, res.Error)
	}
	if res.RowsAffected == 0 {
		return errcode.New(errcode.ErrNotFound)
	}
	return nil
}

// ensureKeyFree 校验参数键存活唯一。
func (s *Service) ensureKeyFree(ctx context.Context, key string, selfID uint64) error {
	if key == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "参数键不能为空")
	}
	q := s.manager.Platform().WithContext(ctx).Model(&model.Config{}).Where("config_key = ?", key)
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

// normalizeStatus 只接受 enabled/disabled，其余按 enabled 处理。
func normalizeStatus(s string) string {
	if s == model.StatusDisabled {
		return model.StatusDisabled
	}
	return model.StatusEnabled
}

// toItem 把存储模型转换为对外条目。
func toItem(c model.Config) Item {
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
