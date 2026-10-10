# internal/model/storage_change.go

sys_storage_change 平台配额历史，继承 BaseModel，TenantID 索引。Action 为 assign/expand，记录套餐、变更前后字节容量、平台操作人和备注。由 Allocate 与账户更新同事务写入，不包含支付或订单语义。
