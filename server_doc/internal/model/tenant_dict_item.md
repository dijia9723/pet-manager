# internal/model/tenant_dict_item.go — t_dict_item

> 脚手架导航：[回到开发者入口](../../../README.md) · [tenant_dict.md](tenant_dict.md) · [租户字典模块](../modules/dict/tenant_service.md) · 对应平台表 [dict_item.md](dict_item.md)

租户字典项表，位于租户 schema（`tenant_<id>`）。与平台 [sys_dict_item](dict_item.md) 结构一致、数据互不相通。

| 字段 | 说明 |
| --- | --- |
| DictID | 所属字典类型 ID，schema 内指向 t_dict |
| Label | 显示标签 |
| Value | 字典项值，同字典下存活唯一（`uk_t_dict_item_value`） |
| Sort | 升序排序 |
| Status | enabled/disabled |
| Remark | 备注 |

字典类型被删除时其下字典项一并软删。
