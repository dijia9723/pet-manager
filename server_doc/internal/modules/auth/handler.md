# internal/modules/auth/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [service.md](service.md) · Swagger 分组「认证」

## 职责

认证模块的 HTTP 层：路由注册 + 请求绑定 + 调 Service + 统一响应。Swagger 注解（全站接口注解的一部分）也在本文件。

## 路由表（Register）

| 分组 | 方法路径 | 权限 | 入参 |
| --- | --- | --- | --- |
| Public | POST `/auth/login` | 无 | `LoginInput`（tenant_code 可空） |
| Public | POST `/auth/refresh` | 无 | `RefreshInput` |
| Authed | POST `/auth/logout` | BearerAuth | 无 |
| Authed | GET `/auth/profile` | BearerAuth | 无 |
| Authed | PUT `/auth/profile` | BearerAuth | `UpdateProfileInput` |
| Authed | POST `/auth/change-password` | BearerAuth | `ChangePasswordInput` |
| Authed | GET `/auth/menus` | BearerAuth | 无 |

## 特殊点

- `RefreshInput` / `UpdateProfileInput` / `ChangePasswordInput` 均为具名 DTO（替代匿名 struct），供 Swagger 生成请求体结构；
- `recordLogin` 在每次登录后采集成功/失败事件，best-effort 投递 loginlog，不影响登录响应；失败时按 tenant_code 有无推断身份；
- 个人资料/改密接口平台与租户账号通用（只校验凭证有效，不挂身份专属中间件）；写操作会被审计中间件记录（不记请求体）。
