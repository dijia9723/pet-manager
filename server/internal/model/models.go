package model

// 本文件汇总两库的模型清单，供 migrate 包 AutoMigrate 使用。
//
// 为什么单独放一个文件：新增表时"改模型文件"和"登记到清单"是两个动作，
// 集中在一处可以一眼看出漏没漏登记。文件顺序刻意与建表顺序一致（被引用的先建）。

// PlatformModels 返回平台公共库（public schema）的全部模型。
func PlatformModels() []any {
	return []any{
		&PlatformUser{},
		&Tenant{},
		&TenantUser{},
		&Permission{},
		&AuditLog{},
		&Config{},
		&Dict{},
		&DictItem{},
		&LoginLog{},
		&Job{},
		&JobLog{},
		&Notification{},
		&NotificationReceipt{},
		&StorageAccount{},
		&StorageResource{},
		&StorageChange{},
		&ManagedServer{},
		&Website{},
	}
}

// TenantModels 返回租户 schema 内的全部模型。
// 新增租户业务表（宠物档案、工单、打印模板等）时只需追加到这里，
// 新开租户会立即带上，存量租户通过 migrate.AllTenants 批量补齐。
func TenantModels() []any {
	return []any{
		&Role{},
		&RolePermission{},
		&UserRole{},
		// 租户自有的参数配置与字典，数据落租户 schema，与平台侧 sys_* 互不相通
		&TenantConfig{},
		&TenantDict{},
		&TenantDictItem{},
		&SchemaVersion{},
	}
}
