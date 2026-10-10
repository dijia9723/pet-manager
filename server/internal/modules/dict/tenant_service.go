// 本文件承载租户侧字典管理（t_dict / t_dict_item）的增删改查。
//
// 与平台侧共用 Item/Entry/Input 契约，区别只在取数入口：
// 平台侧走 manager.Platform()，租户侧只能走请求上下文里的租户事务句柄，
// 按 schema 天然隔离。删除字典类型时在同一事务内级联软删其下字典项。

package dict

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

// —— 租户字典类型 ——

// ListTenantDicts 分页查询当前租户的字典类型。
func (s *Service) ListTenantDicts(ctx context.Context, in ListQuery) (*response.Page[Item], error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	q := tx.Model(&model.TenantDict{})

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

	var rows []model.TenantDict
	if err := q.Order("id desc").Limit(in.Limit()).Offset(in.Offset()).Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	list := make([]Item, 0, len(rows))
	for _, r := range rows {
		list = append(list, toTenantDictItem(r))
	}
	return response.NewPage(list, total), nil
}

// CreateTenantDict 在当前租户下新增字典类型。
func (s *Service) CreateTenantDict(ctx context.Context, in DictInput) (*Item, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	in.Code = strings.TrimSpace(in.Code)
	if err := ensureTenantCodeFree(tx, in.Code, 0); err != nil {
		return nil, err
	}

	d := model.TenantDict{
		Code:   in.Code,
		Name:   in.Name,
		Remark: in.Remark,
		Status: normalizeStatus(in.Status),
	}
	if err := tx.Create(&d).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	item := toTenantDictItem(d)
	return &item, nil
}

// UpdateTenantDict 编辑当前租户的字典类型。
func (s *Service) UpdateTenantDict(ctx context.Context, id uint64, in DictInput) (*Item, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	d, err := loadTenantDict(tx, id)
	if err != nil {
		return nil, err
	}
	in.Code = strings.TrimSpace(in.Code)
	if err := ensureTenantCodeFree(tx, in.Code, id); err != nil {
		return nil, err
	}

	d.Code = in.Code
	d.Name = in.Name
	d.Remark = in.Remark
	d.Status = normalizeStatus(in.Status)
	if err := tx.Save(d).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	item := toTenantDictItem(*d)
	return &item, nil
}

// DeleteTenantDict 软删当前租户的字典类型，并级联软删其下字典项（同一事务）。
func (s *Service) DeleteTenantDict(ctx context.Context, id uint64) error {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return err
	}
	if _, err := loadTenantDict(tx, id); err != nil {
		return err
	}
	err = tx.Transaction(func(gttx *gorm.DB) error {
		if err := gttx.Where("dict_id = ?", id).Delete(&model.TenantDictItem{}).Error; err != nil {
			return err
		}
		return gttx.Delete(&model.TenantDict{}, id).Error
	})
	if err != nil {
		return errcode.Wrap(errcode.ErrDatabase, err)
	}
	return nil
}

// —— 租户字典项 ——

// ListTenantItems 返回当前租户某字典类型下的全部字典项，按 sort、id 升序。
func (s *Service) ListTenantItems(ctx context.Context, dictID uint64) ([]Entry, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := loadTenantDict(tx, dictID); err != nil {
		return nil, err
	}
	var rows []model.TenantDictItem
	if err := tx.
		Where("dict_id = ?", dictID).
		Order("sort asc, id asc").Find(&rows).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	list := make([]Entry, 0, len(rows))
	for _, r := range rows {
		list = append(list, toTenantEntry(r))
	}
	return list, nil
}

// CreateTenantItem 在当前租户指定字典类型下新增字典项。
func (s *Service) CreateTenantItem(ctx context.Context, dictID uint64, in DictItemInput) (*Entry, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := loadTenantDict(tx, dictID); err != nil {
		return nil, err
	}
	in.Value = strings.TrimSpace(in.Value)
	if err := ensureTenantValueFree(tx, dictID, in.Value, 0); err != nil {
		return nil, err
	}

	it := model.TenantDictItem{
		DictID: dictID,
		Label:  in.Label,
		Value:  in.Value,
		Sort:   in.Sort,
		Status: normalizeStatus(in.Status),
		Remark: in.Remark,
	}
	if err := tx.Create(&it).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	entry := toTenantEntry(it)
	return &entry, nil
}

// UpdateTenantItem 编辑当前租户的字典项。
func (s *Service) UpdateTenantItem(ctx context.Context, dictID, itemID uint64, in DictItemInput) (*Entry, error) {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := loadTenantDict(tx, dictID); err != nil {
		return nil, err
	}
	var it model.TenantDictItem
	if err := tx.Where("id = ? AND dict_id = ?", itemID, dictID).First(&it).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}

	in.Value = strings.TrimSpace(in.Value)
	if err := ensureTenantValueFree(tx, dictID, in.Value, itemID); err != nil {
		return nil, err
	}

	it.Label = in.Label
	it.Value = in.Value
	it.Sort = in.Sort
	it.Status = normalizeStatus(in.Status)
	it.Remark = in.Remark
	if err := tx.Save(&it).Error; err != nil {
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	entry := toTenantEntry(it)
	return &entry, nil
}

// DeleteTenantItem 软删当前租户的字典项。
func (s *Service) DeleteTenantItem(ctx context.Context, dictID, itemID uint64) error {
	tx, err := ctxkey.MustTenantDB(ctx)
	if err != nil {
		return err
	}
	res := tx.
		Where("dict_id = ?", dictID).Delete(&model.TenantDictItem{}, itemID)
	if res.Error != nil {
		return errcode.Wrap(errcode.ErrDatabase, res.Error)
	}
	if res.RowsAffected == 0 {
		return errcode.New(errcode.ErrNotFound)
	}
	return nil
}

// —— 内部方法 ——

// loadTenantDict 在当前租户事务内按 ID 读取字典类型，不存在返回 ErrNotFound。
func loadTenantDict(tx *gorm.DB, id uint64) (*model.TenantDict, error) {
	var d model.TenantDict
	if err := tx.First(&d, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDatabase, err)
	}
	return &d, nil
}

// ensureTenantCodeFree 校验租户字典编码在当前 schema 内存活唯一；selfID 用于编辑时排除自身。
func ensureTenantCodeFree(tx *gorm.DB, code string, selfID uint64) error {
	if code == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "字典编码不能为空")
	}
	q := tx.Model(&model.TenantDict{}).Where("code = ?", code)
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

// ensureTenantValueFree 校验租户同字典下取值唯一；selfItemID 用于编辑时排除自身。
func ensureTenantValueFree(tx *gorm.DB, dictID uint64, value string, selfItemID uint64) error {
	if value == "" {
		return errcode.Newf(errcode.ErrInvalidParam, "字典项值不能为空")
	}
	q := tx.Model(&model.TenantDictItem{}).
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

// toTenantDictItem 把租户字典类型存储模型转换为对外条目（与平台侧共用 Item 契约）。
func toTenantDictItem(d model.TenantDict) Item {
	return Item{
		ID:        d.ID,
		Code:      d.Code,
		Name:      d.Name,
		Remark:    d.Remark,
		Status:    d.Status,
		CreatedAt: d.CreatedAt,
	}
}

// toTenantEntry 把租户字典项存储模型转换为对外条目（与平台侧共用 Entry 契约）。
func toTenantEntry(it model.TenantDictItem) Entry {
	return Entry{
		ID:        it.ID,
		DictID:    it.DictID,
		Label:     it.Label,
		Value:     it.Value,
		Sort:      it.Sort,
		Status:    it.Status,
		Remark:    it.Remark,
		CreatedAt: it.CreatedAt,
	}
}
