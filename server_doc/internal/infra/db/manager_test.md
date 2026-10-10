# internal/infra/db/manager_test.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [manager.md](manager.md)

## 职责

不依赖外部服务的纯函数单测，随 `make test` 始终执行：

- `ValidateSchema`：只接受 `^tenant_[0-9]+$`（拒绝 `tenant_`、`tenant_x`、`public`、注入形态等）；
- `SchemaForTenant`：ID → schema 名的确定性推导。

涉及真实连接池/search_path 复位/连接复用不串租户的断言在 [集成测试](../../tests/integration_test.md)。
