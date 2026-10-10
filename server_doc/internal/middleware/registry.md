# internal/middleware/registry.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [bootstrap/app.md](../bootstrap/app.md)

## 职责

有序、可插拔的全局拦截器注册表，解决"中间件顺序被代码书写顺序绑架"的问题。

## 序位表（数字越小越靠前）

| 序位 | 名称 | 文件 |
| --- | --- | --- |
| 10 | recovery | [common.md](common.md) |
| 20 | request_id | [common.md](common.md) |
| 30 | security_headers | [common.md](common.md) |
| 40 | cors | [common.md](common.md) |
| 50 | rate_limit | [common.md](common.md) |
| 60 | access_log | [common.md](common.md) |
| 100 / 110 / 120 / 130 | auth / tenant_context / tenant_scope / rbac | [auth.md](auth.md) · [tenant.md](tenant.md) · [rbac.md](rbac.md) |
| 900 | audit | [audit.md](audit.md) |

## 用法

```go
middleware.NewRegistry().
    Add("name", middleware.OrderXxx, middleware.Xxx()).
    Add(...)
engine.Use(registry.Handlers()...)
```

- 同序位按名字字典序稳定排序；新增拦截器在相邻档位取中间值（65、125）即可；
- `Add` 同名覆盖（便于测试替换）、`Remove` 可临时摘开关；
- `Names()` 按执行顺序返回名称，启动时打日志自证链路。

注意：auth/tenant/rbac 是**路由分组级**中间件（在 [router](../router/router.md) 内挂载），不在此全局表中；序位常量仅作文档与规划用途。
