# internal/seed/storage.go

seedStoragePlans 首次创建平台 storage.plan 字典及 basic 基础 1 GiB、standard 标准 10 GiB、advanced 高级 50 GiB，排序 10/20/30。StorageBytes 保存字节容量，Value 保存等级编码。

已有字典（包括软删除）不复活、不覆盖在线治理、不补回管理员删除的套餐。最小排序启用套餐为未分配租户默认等级。
