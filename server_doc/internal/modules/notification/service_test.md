# notification/service_test.go — 数据库验收

默认仅运行输入校验，无依赖测试。

启用数据库集成测试：在 server 目录执行 `PM_TEST_INTEGRATION=1 PM_TEST_CONFIG=../../../configs/config.yaml go test ./internal/modules/notification -v -count=1`（PowerShell 用 `$env:` 设置变量）。配置路径按测试包工作目录解析。

测试在随机 notification_test_<时间戳> schema 中建独立平台表，结束仅删除本次生成的 schema，不迁移或清理现有业务表。

覆盖：指定租户账号展开、全部租户排除无效目标、平台账号投递、同数字 ID 的平台/租户隔离、跨租户不可读写、同租户员工回执独立、临时同名表遮蔽时仍读取限定的平台表、草稿冻结、已读幂等、处理反馈与统计、撤回与历史回执、无效目标回滚、目标分页搜索、并发发布仅一次成功。

TestTenantPublishingAndAuthorization 覆盖本租户全员/指定账号发布、保存草稿时拒绝越界接收人、发布时重验坏草稿并回滚、接收人搜索隔离、发送方 JSON 字段防伪、平台与租户治理列表隔离、跨租户查看/编辑/发布/撤回/回执拒绝、店内消息处理及统计。测试在请求事务内构造四张临时同名表，验证店内发布仍读写显式限定的真实通知表。

TestLegacySenderMigration 将独立测试通知表恢复为旧结构，再 AutoMigrate，验证已有平台消息的 sender_type=platform、sender_tenant_id=0 默认回填和可见性。
