# internal/model/dict_item.go — sys_dict_item

> 脚手架导航：[回到开发者入口](../../../README.md) · [dict.md](dict.md) · [字典模块](../modules/dict/service.md)

字典项表，挂在某个字典类型下。

| 字段 | 说明 |
| --- | --- |
| DictID | 所属字典类型，索引 |
| Label | 前端显示文案 |
| Value | 实际取值，同字典下存活唯一（`uk_sys_dict_item_value (dict_id, value) WHERE deleted_at IS NULL`） |
| Sort | 排序，升序 |
| Status | enabled/disabled |
| Remark | 备注 |

## 统一存储与附件

StorageBytes（storage_bytes）为 storage.plan 套餐的字节容量，其他平台字典默认 0，最大 1 PiB。Value 为等级编码，Label 为名称，Sort 决定默认等级。
