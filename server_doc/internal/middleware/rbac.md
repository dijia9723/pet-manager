# internal/middleware/rbac.md

> 脚手架导航：[回到开发者入口](../../../README.md) · [router](../router/router.md) · [session.Store](../infra/session/store.md)

## 职责

鉴权链路最后一环：**权限点校验**。

`RBAC.Require(codes...)`：满足任一权限点即放行；不传 codes 表示"登录即可"。

权限集合加载逻辑（每请求幂等，结果写 context）：

- 平台管理员：直接授予通配 `*`，权限点字典是平台自己维护的，无需自我授权；
- 租户账号：先读 Redis（`pm:perm:t:{tid}:{uid}`），未命中调 `PermissionResolver.ResolvePermissions` 回源并回写缓存（10min）。

`HasAny(owned, required)` 支持通配符匹配，属纯函数可独立单测（见 [rbac_test.md](rbac_test.md)）。

## 使用方式

模块注册路由时声明，不自己写中间件：

```go
g.GET("", r.Require("tenant.role.list"), s.handleListRoles)
```

## 缓存失效约定（重要）

- 角色/授权变更 → `InvalidateTenantPermissions(tid)`；
- 权限点字典增删改 → `InvalidateAllPermissions()`（跨租户全量）；
- 缺权限点返回 **HTTP 403 + 业务码 20005**；权限点查询 SQL 必须带 `deleted_at IS NULL`。
