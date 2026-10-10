# internal/modules/storage/module.go

Register 在 r.Platform/r.Tenant 挂载 14 个接口。平台 resources GET/POST/DELETE、download、summary、plans、accounts GET/PUT、changes；租户 resources、download、summary。

storage.list/upload/delete 控制各行为，平台 allocate 控制套餐与扩容。notification.create 也可列资源并上传附件，不自动授权删除或扩容。租户列表和直接下载使用默认 TenantScope；上传、删除和 summary 使用 r.Tenant(path, TransactionInService)，保留身份/状态门禁，服务自行管理短 TenantTx，避免跨 TOS 等待占用外层连接；资源服务再次验证归属。
