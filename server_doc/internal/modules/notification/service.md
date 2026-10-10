# notification/service.go — 通知与回执服务

NewService 接受 db.Manager。平台治理使用平台句柄；租户发布治理、收件箱和回执写入全部使用 ctxkey.MustTenantDB 请求事务。通知、回执、租户和账号表均显式限定平台 schema 表名，防止租户 search_path 中的同名表遮蔽。

Save 仅新建/编辑草稿，规范化标题、正文及去重接收 ID。发送主体固定为当前登录类型及租户 ID；租户保存草稿时仅允许 own_tenant/tenant_users，指定账号必须属于当前租户且为启用态。Publish 对通知行加 UPDATE 锁，重新解析有效租户及启用账号，批量写入接收快照，然后更新 published 状态，任一步失败回滚；重复/并发发布仅一次成功。

managed 限定 sender_type + sender_tenant_id；List、Save、Publish、Revoke、Receipts 均应用该范围。平台管理列表仅包含平台下发消息，租户管理列表仅包含本租户发布消息，跨范围 ID 返回 NotFound。resolveRecipients 对租户消息固定限定 SenderTenantID，混入外部账号或非法接收范围会整体拒绝。租户回执列表与统计额外限定接收人租户。

全部租户与指定租户会下发到租户内所有启用账号。停用、到期和删除租户不参与下发；指定账号或指定租户失效时报参数错误。发布后新增账号不接收历史消息。

Revoke 将 published 改为 revoked，回执保留。Inbox/Unread 只访问当前接收账号且状态为 published 的通知。Acknowledge 对通知加 SHARE 锁、对当前回执加 UPDATE 锁，与撤回互斥；重复已读幂等，处理同时标已读，普通通知和重复处理返回冲突。

List 按通知分组批量统计接收、已读和已处理数量；Receipts 支持状态与租户/账号关键词筛选。Targets 在平台端返回租户、租户账号或平台账号的分页标签；租户端仅允许查询 tenant_users，且只返回本租户启用账号。

范围限于站内消息，未接入短信、邮件或第三方推送。

## 统一存储与附件

NewService 可注入 storage.Service。Input.attachment_ids 在 Save 冻结附件快照，Publish 再校验归属和 ready 状态；无附件的旧通知兼容停用存储。DownloadAttachment 的鉴权逻辑见 attachment.md。
