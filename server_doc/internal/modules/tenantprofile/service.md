# internal/modules/tenantprofile/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · Swagger 分组「租户-店铺资料」

## 职责

租户端"自己的信息"，模块名 `tenant.profile`。只暴露租户可自助维护的基础资料。

## 路由（Register 与 handler 都在本文件）

| 方法路径 | 权限点 |
| --- | --- |
| GET `/tenant/info` | `tenant.profile.view` |
| PUT `/tenant/info` | `tenant.profile.update` |

## 边界

- 可改：name / contact_name / contact_phone / address（`UpdateInput`）；
- **不可改**：status、expire_at——平台运营口径，payload 里带了也不生效；
- 资料表是平台库 `sys_tenant`，更新走 `manager.Platform()`（本接口不碰租户业务表）；
- 更新后失效租户快照缓存（店铺名随缓存下发）。

返回 `Item` 同时带 status/expire_at 只读展示。
