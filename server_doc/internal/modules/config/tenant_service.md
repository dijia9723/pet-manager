# internal/modules/config/tenant_service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [t_config 模型](../../model/tenant_config.md) · 平台侧 [service.md](service.md) · [tenant_handler.md](tenant_handler.md)

## 职责

租户参数（`t_config`）CRUD，与平台侧共用 `Item/Input/ListQuery` 契约。

## 取数入口（硬约定）

- 所有方法第一步都调 `ctxkey.MustTenantDB(ctx)` 取请求级租户事务句柄；
- 该句柄由 TenantScope 中间件注入，`search_path` 已设为 `"tenant_x, public"`，天然按 schema 隔离；
- 拿不到句柄属于编码错误，直接报错，**禁止回退平台连接**。

## 规则

- 与平台侧一致：键存活唯一（`ensureTenantKeyFree`，带 id 自排除）、键允许修改、`normalizeStatus` 归一状态、删除按 RowsAffected 判存在；
- 唯一性校验等内部函数接收外部传入的 `*gorm.DB`，不自己取连接，保证始终运行在当前租户事务内。
