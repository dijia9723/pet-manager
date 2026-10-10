# internal/modules/tenant/purge.go

平台租户立即销毁与定时销毁共用编排。`DestroyInput.confirm_code` 必须精确匹配租户编码；`DestroyResult.pending_resources` 返回等待 TOS 清理的资源数。HTTP 由 handler 校验 `platform.tenant.destroy`，Service 另校验平台身份。

`Destroy` 不要求先删除或等宽限期。`purgeOne` 在平台事务内锁定目标租户；定时路径（confirm=nil）重新确认 deleted 和到期时间，避免旧扫描结果删除已经恢复的租户。校验 schema 正则及 `SchemaForTenant(id)` 一致，拒绝误指向其他租户。

注销所有归属账号的 Redis 会话、失效权限缓存、写 deleted 快照，任一步失败拒绝数据库销毁。随后同一 PostgreSQL 事务执行 `DROP SCHEMA IF EXISTS tenant_<id> CASCADE`，硬删归属账号、租户发送的消息、租户接收回执、套餐变更和租户记录。平台通知、其他租户、审计与登录日志保留。数据库任一步失败，包括 DDL，整体回滚；退出后用独立超时上下文清除临时快照，已注销会话不恢复。

就绪资源转 deleting；uploading 资源保留直到上传完成或半小时超时，防止先删对象后在途 PUT 重建文件。存在未清理资源时保留存储账户与台账，交由 storage.Cleanup 自动重试；资源全部清理后删除账户。无资源时事务内硬删存储账户。TOS 网络操作不占销毁事务。

所有共享平台表显式限定平台 schema，所有删除均有 tenant_id 或租户 ID 条件。该接口不可恢复，前端输入编码确认后才发请求；后端独立校验，不信任前端。
