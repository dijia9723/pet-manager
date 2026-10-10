package seed

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"pet-manager/server/internal/infra/db"
	"pet-manager/server/internal/model"
)

// seedStoragePlans 把存储套餐预置进平台字典（sys_dict/sys_dict_item）：
// 只在首次创建 storage.plan 字典类型时补默认套餐项，重启不覆盖已治理的数据。
func seedStoragePlans(ctx context.Context, m *db.Manager) error {
	return m.Platform().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var d model.Dict
		err := tx.Unscoped().Where("code = ?", "storage.plan").First(&d).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		d = model.Dict{Code: "storage.plan", Name: "存储套餐等级", Status: model.StatusEnabled, Remark: "容量按字节配置；排序最小的启用套餐为新租户默认套餐，已分配配额不随字典修改自动变化"}
		if err := tx.Create(&d).Error; err != nil {
			return err
		}
		items := []model.DictItem{
			{DictID: d.ID, Label: "基础", Value: "basic", Sort: 10, Status: model.StatusEnabled, StorageBytes: 1 << 30},
			{DictID: d.ID, Label: "标准", Value: "standard", Sort: 20, Status: model.StatusEnabled, StorageBytes: 10 << 30},
			{DictID: d.ID, Label: "高级", Value: "advanced", Sort: 30, Status: model.StatusEnabled, StorageBytes: 50 << 30},
		}
		return tx.Create(&items).Error
	})
}
