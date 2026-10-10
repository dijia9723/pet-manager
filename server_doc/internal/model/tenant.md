# internal/model/tenant.go — sys_tenant

> 脚手架导航：[回到开发者入口](../../../README.md) · [租户开通流程](../modules/tenant/service.md)

租户主表（平台公共库），一行代表一个租户及其数据空间。

| 字段 | 列 | 说明 |
| --- | --- | --- |
| BaseModel | id/created_at/… | 见 [base.md](base.md) |
| Name | `name` | 租户/店铺名称 |
| Code | `code` | 租户登录编码，存活唯一（部分唯一索引） |
| SchemaName | `schema_name` | 系统生成 `tenant_<id>`，存活唯一，**不接受用户输入** |
| ContactName / ContactPhone / Address | 同名 | 店铺基础资料，租户可自助维护 |
| Status | `status` | enabled/disabled/deleted，停用或待销毁即拦截全部请求（见 [consts.md](consts.md)） |
| ExpireAt | `expire_at` | 可空；空 = 永久有效 |
| PurgeAt | `purge_at` | 可空；两段式删除的物理销毁时间（普通索引，与 status 组成 `idx_sys_tenant_purge`） |

## 约束

- 开通编排（建租户 → 建 schema 建表 → 预置角色 → 建主账号）必须在同一事务原子完成，见 [tenant.Service](../modules/tenant/service.md)；
- 状态/到期时间属平台运营口径，租户端 [tenantprofile](../modules/tenantprofile/service.md) 改不了；
- **两段式删除**：平台点删除后进入 `deleted` 待销毁态（保留行 + 写 purge_at=now+N 天 + 踢全部会话），到期由内置任务 `tenant_purge` 物理 DROP schema 并硬删记录，宽限天数取参数 `tenant.purge_after_days`（默认 7），见 [tenant.Service 删除/销毁](../modules/tenant/service.md) 与 [任务模块](../modules/job/service.md)。
