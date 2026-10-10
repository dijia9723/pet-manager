# internal/model/dict.go — sys_dict

> 脚手架导航：[回到开发者入口](../../../README.md) · [dict_item.md](dict_item.md) · [字典模块](../modules/dict/service.md)

字典类型表，一个类型下挂若干 [字典项](dict_item.md)，供全站下拉/枚举统一取值。
租户自有字典在租户 schema 的 [t_dict](tenant_dict.md)，两表结构一致、数据互不相通。

| 字段 | 说明 |
| --- | --- |
| Code | 字典编码，存活唯一（`uk_sys_dict_code`） |
| Name | 显示名 |
| Remark | 备注 |
| Status | enabled/disabled |

删除字典类型时其下字典项一并软删（业务层级联）。
