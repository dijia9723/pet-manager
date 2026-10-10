# internal/model/storage_account.go

sys_storage_account 平台配额台账，继承 BaseModel。TenantID=0 为平台公共空间，无业务配额上限；存活 tenant_id 部分唯一。PlanID 来自平台 storage.plan 字典，PlanName/BaseBytes 为分配时快照，ExtraBytes 为追加扩容，UsedBytes 包含 uploading/ready/deleting 占用。

账户首次使用按字典排序最小的启用套餐初始化；修改字典不自动改变已分配配额。存储服务对账户行加 UPDATE 锁，串行化上传预占、配额变化和释放。对象成功清理后才释放容量。
