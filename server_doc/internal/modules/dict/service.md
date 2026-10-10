# internal/modules/dict/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [sys_dict](../../model/dict.md) · [sys_dict_item](../../model/dict_item.md) · 租户侧 [tenant_service.md](tenant_service.md)

## 职责

平台字典类型与字典项（`sys_dict` / `sys_dict_item`）的全部业务，仅操作平台库。
租户侧（`t_dict` / `t_dict_item`）的同构业务在 [tenant_service.md](tenant_service.md)，两侧共用本文件的 DTO 与 `normalizeStatus`。

## 规则

- 字典 Code 存活唯一；字典项 `(dict_id, value)` 存活唯一，冲突返业务错误；
- 删除字典类型时其下字典项按业务层级联软删；
- 字典项列表按 `sort asc, id asc` 返回全量（不分页）；
- 增删改均做存在性校验，资源不存在统一返 10002；
- 字典属于配置数据，不涉及权限缓存失效。

## DTO

`Item`（字典类型）、`Entry`（字典项）、`DictInput`、`DictItemInput`、`ListQuery`（分页 + keyword），平台侧与租户侧共用。

## 统一存储与附件

Entry/DictItemInput 增加 storage_bytes，平台 DictItem 持久化，租户普通字典忽略。storage.plan 容量必须大于 0，绑定上限 1 PiB。前端以 GiB 展示/录入，按字节写入；修改不自动改变已分配配额。
