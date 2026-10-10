# internal/router/router.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [middleware](../middleware/registry.md) · [bootstrap/app.md](../bootstrap/app.md)

## 职责

路由装配与模块挂载。业务模块只声明"挂在哪个命名空间、要什么权限点"，鉴权链路由这里统一施加。

## 四组路由命名空间

| 方法 | 路径前缀 | 中间件链 | 典型接口 |
| --- | --- | --- | --- |
| `Public(path)` | `/api/v1` | 无 | 登录、刷新 |
| `Authed(path)` | `/api/v1` | Auth（不区分身份） | 登出、个人信息、菜单 |
| `Platform(path)` | `/api/v1/platform` | Auth(platform) | 平台运营全部接口 |
| `Tenant(path)` | `/api/v1/tenant` | Auth(tenant) → TenantGuard → TenantScope | 租户业务全部接口 |

权限点通过 `r.Require(codes...)` 在模块内按路由声明（RBAC 中间件，分组链的最后一环）。

## Module 接口

```go
type Module interface {
    Name() string              // 模块名，用于启动日志
    Register(r *Router)        // 声明路由
}
```

`Mount(modules...)` 按传入顺序注册并返回名称列表。新增模块只需实现该接口并在 [bootstrap](../bootstrap/app.md) 的 `modules` 列表登记。

## 服务管理的短租户事务

Tenant(path, TransactionInService) 保留 Auth(tenant) 与 TenantGuard，省略空的外层 TenantScope。仅用于资源上传、删除、配额账户查询等自行使用 Manager.TenantTx 的存储接口，防止并发请求先占满连接池，再等待第二个连接而耗尽连接。服务必须通过 TenantTx 和显式 schema 表名访问租户归属台账；不得调用租户裸 Platform()。普通查询、消息管理和附件鉴权仍使用默认请求事务。
