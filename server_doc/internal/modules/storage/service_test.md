# internal/modules/storage/service_test.go

PM_TEST_INTEGRATION=1 / PM_TEST_CONFIG 启用随机 storage_test_* PostgreSQL schema；仅创建和 cleanup 本测试表，对象服务注入内存故障。

覆盖默认容量、文件限制、租户列表/下载/删除隔离、自扩容拒绝、停用套餐、升级保留扩容、变更记录、失败上传补偿、失败删除容量保留及后台重试、并发上传不能超额。

并发上传测试将连接池限制为 1 个连接，并通过无外层请求事务的短事务入口调用，验证上传与 summary 无需等待第二个连接。

真实通知服务联测附件去重、收件箱 JSON 元数据、接收快照下载、非接收人/错误资源 ID 拒绝、撤回停止签发、删除附件阻止草稿发布、平台与租户分别发送附件。
