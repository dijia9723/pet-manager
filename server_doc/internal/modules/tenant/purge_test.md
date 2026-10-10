# internal/modules/tenant/purge_test.go

设置 `PM_TEST_INTEGRATION=1` 和 `PM_TEST_CONFIG` 启用真实 PostgreSQL/Redis 集成测试。仅在独立随机平台 schema、纳秒 ID 对应的租户 schema、独立 Redis 前缀下构造和销毁测试数据，结束清理，不操作存量租户或真实 TOS 对象。

覆盖：启用租户立即硬删、业务 schema 销毁、平台通知和其他租户隔离、保留审计、注销账号会话、用户 ID 1 与 10 前缀隔离、错误确认/非平台身份/schema 错配拒绝、重复销毁、注入数据库错误后 DDL 和记录回滚、定时到期复核与立即跳过宽限期。

内存对象存储配合真实台账测试销毁发生在上传途中：在途文件不提前删除，PUT 完成后转 deleting；TOS 删除失败保留容量并自动重试，最后一个对象成功清理后删除存储账户，其他租户文件保留，旧租户上下文不能重新上传。
