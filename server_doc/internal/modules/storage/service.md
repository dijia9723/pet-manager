# internal/modules/storage/service.go

Service 依赖 db.Manager、objectstore.Store、TOSConfig。普通租户读走 ctxkey.MustTenantDB。跨网络对象操作的路由使用 TransactionInService，不开启空外层事务；服务采用独立 Manager.TenantTx 写台账，SET LOCAL search_path，并显式 schema 限定平台表。平台治理和后台任务走平台连接，租户不得裸 Platform()。

Summary/Accounts 懒初始化账户；Plans 从平台 storage.plan 读取启用且容量有效的字典项。Allocate 支持 assign（保留追加容量）和 expand，拒绝总容量小于已占用或超过 1 PiB，原子记录 StorageChange。

Upload 先锁账户原子预占实际字节容量并提交 uploading 台账，再写 TOS，成功 ready，失败 deleting 并补偿。独立提交确保外层 HTTP 回滚不会丢失清理记录。取消消息表单不会删除已上传文件，可在资源页清理。

租户上传预占、上传完成、缺失账户初始化和平台套餐分配先取得租户行 SHARE 锁，与租户销毁的 UPDATE 锁串行化。预占要求租户仍存在且 enabled；上传完成时发现租户已删除/销毁则转 deleting 并返回冲突，保留已提交台账用于对象回收，防止在途上传留下孤儿文件。已销毁租户不能重新分配套餐。

Delete 按归属锁资源并提交 deleting，再删对象；cleanOne 用账户锁、资源锁幂等软删和释放。故障保留占用。每 5 分钟 Cleanup 重试 deleting、条件抢占半小时未完成上传，避免删除已完成上传；每轮最多 100 个。

租户销毁保留待清理资源与存储账户。cleanOne 删除对象并释放容量后，若租户已不存在且无剩余活动资源，则硬删该存储账户。平台存储与其他租户资源不受影响。

List 平台可按租户筛选，own=true 仅自身资源；租户忽略伪造 tenant_id。Download 仅本租户资源，平台可管理全部。Attachments 仅当前主体自己的 ready 资源，最多 10 个，去重冻结元数据，SHARE 锁与删除互斥。AttachmentDownload 仅供通知模块验证接收快照/管理权限后调用，禁止另挂路由。

附件资源被删除或正在清理时，AttachmentDownload 返回 NotFound 和明确的不可下载提示，历史消息仍保留快照。
