# internal/tests/integration_test.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [Makefile 命令](../../../server/Makefile)

## 职责

真连 PostgreSQL + Redis 的端到端集成测试，验证多租户底座的**硬约束**：

- 租户间数据互不可见（schema 隔离）；
- 越权访问返回 403 + 业务码 20005；
- 平台连接直接查询 `t_role` 必须报错；
- 事务后 `current_schema` 复位、连接池不串租户。

## 运行方式

默认跳过（不阻塞无依赖环境的 `make test`），需显式开启：

```bash
# 需要可用的 PG 与 Redis，配置走 PM_ 环境变量或 config.yaml
PM_TEST_INTEGRATION=1 go test ./internal/tests/... -count=1 -v
# 或 make test-int
```

## 约定

- 单测（不依赖外部服务）与被测包放同目录（如 `db/manager_test.go`、`middleware/rbac_test.go`）；
- 涉及真实中间件、真实隔离性的断言放本目录，避免 CI 在无依赖时失败；
- 测试产生的租户 schema 用 `DropTenantSchema` 清理。
