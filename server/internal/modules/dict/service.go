// Package dict 承载字典管理（平台侧 sys_dict/sys_dict_item 与租户侧 t_dict/t_dict_item）的增删改查。
//
// 一个字典类型（Dict）下挂若干字典项（DictItem），供下拉/枚举统一取值。
// 平台侧与租户侧共用 Item/Entry/Input 契约，区别只在取数入口（manager.Platform() vs
// 请求上下文里的租户事务句柄），见 tenant_service.go。
// 删除字典类型时级联软删其下字典项，避免残留孤儿项。
package dict

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

// Item 是对外的字典类型条目。
type Item struct {
	ID        uint64    `json:"id"`         // 主键
	Code      string    `json:"code"`       // 字典编码（存活唯一）
	Name      string    `json:"name"`       // 字典名称
	Remark    string    `json:"remark"`     // 备注
	Status    string    `json:"status"`     // 状态：enabled/disabled
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// Entry 是对外的字典项条目。
type Entry struct {
	StorageBytes int64     `json:"storage_bytes"` // 存储套餐容量（字节），仅 storage.plan 字典使用
	ID           uint64    `json:"id"`            // 主键
	DictID       uint64    `json:"dict_id"`       // 所属字典类型 ID
	Label        string    `json:"label"`         // 显示文本
	Value        string    `json:"value"`         // 取值（同字典下唯一）
	Sort         int       `json:"sort"`          // 排序（升序）
	Status       string    `json:"status"`        // 状态：enabled/disabled
	Remark       string    `json:"remark"`        // 备注
	CreatedAt    time.Time `json:"created_at"`    // 创建时间
}

// DictInput 是新增/编辑字典类型的入参。
type DictInput struct {
	Code   string `json:"code" binding:"required,max=64"` // 字典编码
	Name   string `json:"name" binding:"required,max=64"` // 字典名称
	Remark string `json:"remark" binding:"max=255"`       // 备注
	Status string `json:"status"`                         // 状态，非 disabled 一律按 enabled
}

// DictItemInput 是新增/编辑字典项的入参。
type DictItemInput struct {
	StorageBytes int64  `json:"storage_bytes" binding:"gte=0,lte=1125899906842624"` // 存储套餐容量（字节，最大 1PiB）
	Label        string `json:"label" binding:"required,max=64"`                    // 显示文本
	Value        string `json:"value" binding:"required,max=128"`                   // 取值
	Sort         int    `json:"sort"`                                               // 排序
	Status       string `json:"status"`                                             // 状态，非 disabled 一律按 enabled
	Remark       string `json:"remark" binding:"max=255"`                           // 备注
}

// ListQuery 是字典类型列表查询参数。
type ListQuery struct {
	pagination.Query
	Keyword string `form:"keyword"` // 编码/名称关键字
	Status  string `form:"status"`  // 按状态筛选
}

// Service 承载平台侧字典（sys_dict/sys_dict_item）的读写；租户侧见 tenant_service.go。
type Service struct {
	manager *db.Manager
}

// NewService 构造字典服务。
func NewService(manager *db.Manager) *Service {
	return &Service{manager: manager}
}

// —— 字典类型 ——

// ListDicts 分页查询字典类型。
func (s *Service) ListDicts(ctx context.Context, in ListQuery) (*response.Page[Item], error) {
	q := s.manager.Platform().WithContext(ctx).Model(&model.Dict{})

	if kw := strings.TrimSpace(in.Keyword); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("code ILIKE ? OR name ILIKE ?", like, like)
	}
	if st := strings.TrimSpace(in.Status); st != "" {
		q = q.Where("status = ?", st)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	var rows []model.Dict
	if err := q.Order("id desc").Limit(in.Limit()).Offset(in.Offset()).Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	list := make([]Item, 0, len(rows))
	for _, r := range rows {
		list = append(list, toItem(r))
	}
	return response.NewPage(list, total), nil
}

// CreateDict 新增字典类型。
func (s *Service) CreateDict(ctx context.Context, in DictInput) (*Item, error) {
	in.Code = strings.TrimSpace(in.Code)
	if err := s.ensureCodeFree(ctx, in.Code, 0); err != nil {
		return nil, err
	}

	d := model.Dict{
		Code:   in.Code,
		Name:   in.Name,
		Remark: in.Remark,
		Status: normalizeStatus(in.Status),
	}
	if err := s.manager.Platform().WithContext(ctx).Create(&d).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	item := toItem(d)
	return &item, nil
}

// UpdateDict 编辑字典类型。
func (s *Service) UpdateDict(ctx context.Context, id uint64, in DictInput) (*Item, error) {
	d, err := s.loadDict(ctx, id)
	if err != nil {
		return nil, err
	}
	in.Code = strings.TrimSpace(in.Code)
	if err := s.ensureCodeFree(ctx, in.Code, id); err != nil {
		return nil, err
	}

	d.Code = in.Code
	d.Name = in.Name
	d.Remark = in.Remark
	d.Status = normalizeStatus(in.Status)
	if err := s.manager.Platform().WithContext(ctx).Save(d).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	item := toItem(*d)
	return &item, nil
}

// DeleteDict 软删字典类型，并级联软删其下字典项（同一事务）。
func (s *Service) DeleteDict(ctx context.Context, id uint64) error {
	if _, err := s.loadDict(ctx, id); err != nil {
		return err
	}
	err := s.manager.Platform().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("dict_id = ?", id).Delete(&model.DictItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Dict{}, id).Error
	})
	if err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	return nil
}

// —— 字典项 ——

// ListItems 返回某字典类型下的全部字典项，按 sort、id 升序（不分页，字典项数量有限）。
func (s *Service) ListItems(ctx context.Context, dictID uint64) ([]Entry, error) {
	if _, err := s.loadDict(ctx, dictID); err != nil {
		return nil, err
	}
	var rows []model.DictItem
	if err := s.manager.Platform().WithContext(ctx).
		Where("dict_id = ?", dictID).
		Order("sort asc, id asc").Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	list := make([]Entry, 0, len(rows))
	for _, r := range rows {
		list = append(list, toEntry(r))
	}
	return list, nil
}

// CreateItem 在指定字典类型下新增字典项。
func (s *Service) CreateItem(ctx context.Context, dictID uint64, in DictItemInput) (*Entry, error) {
	d, err := s.loadDict(ctx, dictID)
	if err != nil {
		return nil, err
	}
	if d.Code == "storage.plan" && in.StorageBytes <= 0 {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "存储套餐容量必须大于 0")
	}
	in.Value = strings.TrimSpace(in.Value)
	if err := s.ensureValueFree(ctx, dictID, in.Value, 0); err != nil {
		return nil, err
	}

	it := model.DictItem{
		StorageBytes: in.StorageBytes,
		DictID:       dictID,
		Label:        in.Label,
		Value:        in.Value,
		Sort:         in.Sort,
		Status:       normalizeStatus(in.Status),
		Remark:       in.Remark,
	}
	if err := s.manager.Platform().WithContext(ctx).Create(&it).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	entry := toEntry(it)
	return &entry, nil
}

// UpdateItem 编辑字典项。
func (s *Service) UpdateItem(ctx context.Context, dictID, itemID uint64, in DictItemInput) (*Entry, error) {
	d, err := s.loadDict(ctx, dictID)
	if err != nil {
		return nil, err
	}
	if d.Code == "storage.plan" && in.StorageBytes <= 0 {
		return nil, errcode.Newf(errcode.ErrInvalidParam, "存储套餐容量必须大于 0")
	}
	tx := s.manager.Platform().WithContext(ctx)
	var it model.DictItem
	if err := tx.Where("id = ? AND dict_id = ?", itemID, dictID).First(&it).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	in.Value = strings.TrimSpace(in.Value)
	if err := s.ensureValueFree(ctx, dictID, in.Value, itemID); err != nil {
		return nil, err
	}

	it.Label = in.Label
	it.StorageBytes = in.StorageBytes
	it.Value = in.Value
	it.Sort = in.Sort
	it.Status = normalizeStatus(in.Status)
	it.Remark = in.Remark
	if err := tx.Save(&it).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	entry := toEntry(it)
	return &entry, nil
}

// DeleteItem 软删字典项。
func (s *Service) DeleteItem(ctx context.Context, dictID, itemID uint64) error {
	res := s.manager.Platform().WithContext(ctx).
		Where("dict_id = ?", dictID).Delete(&model.DictItem{}, itemID)
	if res.Error != nil {
		return errcode.Wrap(errcode.ErrDatabase, res.Error)
	}
	if res.RowsAffected == 0 {
		return errcode.New(errcode.ErrNotFound)
	}
	return nil
}

// —— 内部方法 ——

// loadDict 按 ID 读取字典类型，不存在返回 ErrNotFound。
func (s *Service) loadDict(ctx context.Context, id uint64) (*model.Dict, error) {
	var d model.Dict
	if err := s.manager.Platform().WithContext(ctx).First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return &d, nil
}

// ensureCodeFree 校验字典编码存活唯一；selfID 用于编辑时排除自身。
func (s *Service) ensureCodeFree(ctx context.Context, code string, selfID uint64) error {
	if code == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "字典编码不能为空")
	}
	q := s.manager.Platform().WithContext(ctx).Model(&model.Dict{}).Where("code = ?", code)
	if selfID > 0 {
		q = q.Where("id <> ?", selfID)
	}
	var dup int64
	if err := q.Count(&dup).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if dup > 0 {
		return errcode.Newf(errcode.ErrConflict, "字典编码已存在")
	}
	return nil
}

// ensureValueFree 校验同字典下字典项取值唯一；selfItemID 用于编辑时排除自身。
func (s *Service) ensureValueFree(ctx context.Context, dictID uint64, value string, selfItemID uint64) error {
	if value == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "字典项值不能为空")
	}
	q := s.manager.Platform().WithContext(ctx).Model(&model.DictItem{}).
		Where("dict_id = ? AND value = ?", dictID, value)
	if selfItemID > 0 {
		q = q.Where("id <> ?", selfItemID)
	}
	var dup int64
	if err := q.Count(&dup).Error; err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	if dup > 0 {
		return errcode.Newf(errcode.ErrConflict, "同字典下该值已存在")
	}
	return nil
}

func normalizeStatus(s string) string {
	if s == model.StatusDisabled {
		return model.StatusDisabled
	}
	return model.StatusEnabled
}

// toItem 把字典类型存储模型转换为对外条目。
func toItem(d model.Dict) Item {
	return Item{
		ID:        d.ID,
		Code:      d.Code,
		Name:      d.Name,
		Remark:    d.Remark,
		Status:    d.Status,
		CreatedAt: d.CreatedAt,
	}
}

// toEntry 把字典项存储模型转换为对外条目。
func toEntry(it model.DictItem) Entry {
	return Entry{
		StorageBytes: it.StorageBytes,
		ID:           it.ID,
		DictID:       it.DictID,
		Label:        it.Label,
		Value:        it.Value,
		Sort:         it.Sort,
		Status:       it.Status,
		Remark:       it.Remark,
		CreatedAt:    it.CreatedAt,
	}
}
