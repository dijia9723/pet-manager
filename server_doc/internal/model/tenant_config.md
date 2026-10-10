# internal/model/tenant_config.go — t_config

> 脚手架导航：[回到开发者入口](../../../README.md) · [租户参数配置模块](../modules/config/tenant_service.md) · 对应平台表 [config.md](config.md)

租户自有参数表，位于租户 schema（`tenant_<id>`），承载本店"不改代码即可调整"的业务参数。
与平台 [sys_config](config.md) 结构一致、数据互不相通：平台不感知租户参数，租户之间按 schema 隔离。

| 字段 | 说明 |
| --- | --- |
| ConfigKey | 参数键，schema 内存活唯一（`uk_t_config_key`） |
| Name | 显示名 |
| Value | text，按 Type 解释 |
| GroupKey | 分组列，**刻意不用 group**（PostgreSQL 保留字） |
| Type | string / number / bool / json |
| Remark | 备注 |
| Status | enabled/disabled |

只能通过 `ctxkey.MustTenantDB(ctx)` 拿到的请求级租户事务句柄访问，禁止用平台连接直连。
