# internal/model/config.go — sys_config

> 脚手架导航：[回到开发者入口](../../../README.md) · [参数配置模块](../modules/config/service.md)

平台系统级参数表，承载"不改代码即可调整"的配置（站点名称、开关等），不承载租户业务参数。
租户自有参数在租户 schema 的 [t_config](tenant_config.md)，两表结构一致、数据互不相通。

| 字段 | 说明 |
| --- | --- |
| ConfigKey | 参数键，存活唯一（`uk_sys_config_key`） |
| Name | 显示名 |
| Value | text，按 Type 解释 |
| GroupKey | 分组列，**刻意不用 group**（PostgreSQL 保留字） |
| Type | string / number / bool / json |
| Remark | 备注 |
| Status | enabled/disabled |

参数变更不涉及权限字典，无需失效权限缓存。
