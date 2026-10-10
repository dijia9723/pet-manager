# internal/model/tenant_dict.go — t_dict

> 脚手架导航：[回到开发者入口](../../../README.md) · [tenant_dict_item.md](tenant_dict_item.md) · [租户字典模块](../modules/dict/tenant_service.md) · 对应平台表 [dict.md](dict.md)

租户字典类型表，位于租户 schema（`tenant_<id>`），一个类型下挂若干 [租户字典项](tenant_dict_item.md)，
承载本店业务枚举（宠物品种、服务分类等）。与平台 [sys_dict](dict.md) 结构一致、数据互不相通。

| 字段 | 说明 |
| --- | --- |
| Code | 字典编码，schema 内存活唯一（`uk_t_dict_code`） |
| Name | 显示名 |
| Remark | 备注 |
| Status | enabled/disabled |

删除字典类型时其下字典项在同一事务内一并软删（业务层级联）。
