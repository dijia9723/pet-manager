# internal/middleware/tenant.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [db.Manager](../infra/db/manager.md) · [router](../router/router.md)

## 职责

租户请求的两道门禁，挂在 `Router.Tenant(path)` 分组上，顺序固定：

### 1. TenantGuard.Context（序位 110）

从 context 取 tenant_id → `TenantResolver.TenantByID`（优先 Redis 快照）→ 校验：

- 租户存在（`20006`）；
- `status=enabled`（`20007`）；
- 未到期（`20008`，空到期时间视为永久）；
- 快照 schema 过 `db.ValidateSchema`，并**以数据库为准回填** context，防凭证陈旧值。

### 2. TenantScope（序位 120）

- `BeginTenant` 开请求级事务并 `SET LOCAL search_path = tenant_x, public`；
- 事务句柄注入 context（`ctxkey.WithTenantDB`），租户域仓储只能用它取数；
- 请求结束：响应状态 <400 提交，否则回滚；panic 也回滚并继续抛出；
- 连接归还连接池时 search_path 自动复位，不串租户。

## 约束

- 租户域**禁止**裸连接访问，仓储入口用 `ctxkey.MustTenantDB(ctx)`，拿不到事务直接报错；
- 事务持有到请求结束，租户域接口不适合超长任务（大文件上传、SSE 请走非租户域路由）。
