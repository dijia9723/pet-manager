# internal/pkg/ctxkey/ctxkey.go

> 脚手架导航：[回到开发者入口](../../../../README.md)

## 职责

集中管理 `context.Context` 的键，杜绝魔法字符串散落各模块。键类型为私有 `type key string`，不与其他包冲突。

## 键与读写函数

| 键 | 写入点 | 读取方 |
| --- | --- | --- |
| request_id | RequestID 中间件 | 日志、审计 |
| user_id / username / user_type | `WithUser`（Auth） | 全部鉴权后业务 |
| session_id | `WithSession`（Auth） | 登出注销 |
| tenant_id / tenant_code / tenant_schema | `WithTenant`（Auth + Guard 回填） | 租户域全部业务 |
| permissions | RBAC load | 业务内细粒度判断 |
| tenant_db | TenantScope 注入的请求级事务 | 租户域仓储（唯一合法取数入口） |

## 关键约束

- `user_type` 常量：`UserTypePlatform` / `UserTypeTenant`；
- `MustTenantDB(ctx)` 拿不到事务句柄直接报内部错误，**不回退平台连接**——说明请求漏挂 TenantScope，属编码错误；
- 业务代码只读这些值，写入只允许出现在中间件。
