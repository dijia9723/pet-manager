# internal/modules/notification/attachment.go

validateAttachments 通过 storage.Service 校验当前主体归属、ready 状态和最多 10 个附件，冻结名称/字节大小。Save 和 Publish 均校验，旧消息无附件兼容。

DownloadAttachment 管理模式仅发布主体范围，由路由加 notification.list；接收模式要求 user_type+tenant_id+user_id 接收快照及 published 状态。附件还必须属于该消息快照，然后签发临时下载地址，不授权其他资源访问。撤回停止签发新 URL，已签发 URL 在 presign_ttl 内可能有效；删除资源后下载 NotFound。
