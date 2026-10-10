# notification.go — 平台与租户通知

`Notification` 对应平台库 `sys_notification`，继承 BaseModel，由 PlatformModels 自动迁移。
标题、正文、类型（announcement/maintenance/business）、优先级（normal/urgent）、是否需要处理、接收范围和 ID 列表构成通知草稿。TargetIDs 使用 jsonb 序列化。

状态为 draft → published → revoked；仅 draft 可编辑，发布时冻结内容和接收范围，记录发送账号和发布时间。发布与编辑使用行锁，发布后的撤回不清理回执。

平台接收范围：all_tenants、tenants、tenant_users、platform_users。租户接收范围仅 own_tenant（本租户全部启用账号）及 tenant_users（本租户指定启用账号）。所有接收人由 [notification 服务](../modules/notification/service.md) 在发布事务中解析。

SenderType、SenderTenantID 与 SenderID 均从鉴权上下文写入，不接受客户端指定。通知发布治理按 SenderType + SenderTenantID 隔离，租户可管理本租户发布消息，平台治理仅管理平台下发消息。发送主体复合索引 idx_notification_sender 支持管理列表。

SenderType 默认 platform、SenderTenantID 默认 0，AutoMigrate 补列时将历史平台通知保留为平台归属，无需新增租户表或调整租户 schema 版本。

## 统一存储与附件

Attachments 以 jsonb 保存 Attachment 资源 ID/名称/大小/类型快照，旧通知可 NULL/空数组。引用不重复扣容量，无固定公开 URL，需经通知鉴权接口获取临时地址。
