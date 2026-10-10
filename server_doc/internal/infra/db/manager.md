# internal/infra/db/manager.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [migrate/migrate.md](../../migrate/migrate.md) · [middleware/tenant.md](../../middleware/tenant.md)

## 职责

**多租户数据访问的唯一入口**。全项目只有一个 GORM 连接池，平台数据与租户数据通过 schema 隔离。

## 核心规则（硬约束）

1. 平台库查询用 `Platform()`；租户业务数据**必须**走 `TenantTx` / `BeginTenant`；
2. 租户事务内执行 `set_config('search_path', 'tenant_x, public', true)`，`is_local=true` 等价 `SET LOCAL`，事务结束连接自动复位——**连接池复用不会串租户**；
3. schema 名由 `SchemaForTenant(id)` 生成（`tenant_<id>`）并经正则 `^tenant_[0-9]+$` 校验，不接受任何用户输入；
4. 租户域代码禁止调用 `Platform()` 直连，是代码评审硬性检查点。

## API 速查

| 方法 | 用途 |
| --- | --- |
| `New(cfg, log)` | 建唯一连接池（SkipDefaultTransaction，时间取本地时区） |
| `Platform()` | 平台公共库句柄（仅平台域/迁移用） |
| `TenantTx(ctx, schema, fn)` | 后台任务用：自动开事务/提交/回滚/复位 |
| `BeginTenant` / `EndTenant` | 请求级事务（由 TenantScope 中间件持有到请求结束） |
| `CreateTenantSchema` / `DropTenantSchema` | 开通/注销租户，DDL 幂等 |
| `ValidateSchema` / `SchemaForTenant` | schema 名校验与推导，全项目唯一实现 |

## 教训

- 曾因 `set_config(..., false)`（会话级）导致连接池串租户；必须用 `true`（事务级）。

配套测试：[manager_test.md](manager_test.md)。

## 网站分配台账例外

hosting 的 Website 存在平台表；租户路由使用 TransactionInService，由服务对认证 TenantID 强制过滤。跨 SSH/SFTP 请求不持有空外层 schema 事务；不允许将此例外扩展到普通租户业务表。
