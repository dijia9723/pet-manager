# 集成测试

> [开发者入口](../../../../README.md) · [任务管理](service.md) · [执行器测试](executor_test.md)

## 触发方式

默认跳过；设置环境变量 `PM_TEST_JOB_CONFIG` 为配置文件**绝对路径**后执行：

```bash
PM_TEST_JOB_CONFIG=/abs/path/config.yaml go test ./internal/modules/job -run TestJobIntegration
```

## 隔离策略

- 仅在**随机独立 schema**（`job_test_<随机>`）与独立 Redis 前缀中创建测试数据，结束后清理，不影响现有数据。
- 不启动应用、不迁移公共表、不注册任何真实业务方法；使用测试方法与本地 HTTP 服务，**不启动真实业务调度**。

## 覆盖范围

调度与并发锁、任务 CRUD、执行日志落库、HTTP/shell 执行链路在真实 PostgreSQL + Redis 下的端到端行为。

## 约定

命名空间格式受正则约束（`^job_test_[a-z2-7]+$`），非法即失败，避免误连生产库/schema。
