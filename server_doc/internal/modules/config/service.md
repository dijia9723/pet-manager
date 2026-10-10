# internal/modules/config/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [sys_config 模型](../../model/config.md) · 租户侧 [tenant_service.md](tenant_service.md)

## 职责

平台参数（`sys_config`）CRUD，仅操作平台库，无缓存（或由读取方自行缓存），不涉及权限字典。
租户侧（`t_config`）的同构 CRUD 在 [tenant_service.md](tenant_service.md)，两侧共用本文件的 DTO 与 `normalizeStatus`。

## 规则

- 键存活唯一：`ensureKeyFree` 显式查存活行（带 id 自排除），重复返冲突；
- 参数键允许修改，但改后仍须存活唯一；
- `normalizeStatus` 只接受 enabled/disabled，其余按 enabled；
- type 入参由 binding 限定 `oneof=string number bool json`；
- 删除按 RowsAffected 判断资源是否存在（不存在返 10002）。

## DTO

`Item`（对外条目）、`Input`（新增/编辑共用）、`ListQuery`（分页 + keyword/group_key/status），平台侧与租户侧共用。
