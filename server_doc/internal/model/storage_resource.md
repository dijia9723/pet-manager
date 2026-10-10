# internal/model/storage_resource.go

sys_storage_resource 平台资源台账，继承 BaseModel。所有对象写平台 TOS；TenantID 由登录上下文决定，0 为平台。Name/SizeBytes/ContentType/Uploader 供展示；Bucket/ObjectKey 不序列化到 HTTP，活跃 bucket+key 部分唯一。

状态 uploading→ready/deleting、ready→deleting→软删除。随机 key 不包含原文件名。Attachment 是消息内 JSON 快照，保存资源 ID/名称/大小/内容类型，无永久公开 URL。删除资源保留历史消息快照，但不能下载；复用不重复扣配额。
