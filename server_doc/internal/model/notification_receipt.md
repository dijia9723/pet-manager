# notification_receipt.go — 通知接收回执

`NotificationReceipt` 对应平台库 `sys_notification_receipt`，继承 BaseModel，记录发布时的接收人快照（账号类型、租户 ID、账号 ID、账号名及租户名）。

ReadAt 与 ProcessedAt/Feedback 分别保存已读及处理结果。普通通知无需反馈；需要处理的通知只能提交一次结果。同租户不同账号状态互相独立。

存活唯一键为 notification_id + user_type + tenant_id + user_id，通过 migrate 的部分唯一索引保证。索引 idx_notification_inbox 支持账号收件箱。

接收查询和更新必须同时限定 user_type、tenant_id、user_id，不允许仅凭通知 ID 修改。撤回后回执保留供平台追踪。
