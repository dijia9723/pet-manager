# internal/modules/permission/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [sys_permission 模型](../../model/permission.md) · [seed 权限字典](../../seed/seed.md)

## 职责

权限点字典的在线治理，模块名 `platform.permission`，仅操作平台库。

## 能力与约定

- `Tree`：返回全部权限点组成的树（menu/button 混合，按 parent_id 组装，sort 排序）；
- `Create/Update/Delete`：在线维护名称、路径、图标、排序、父子关系等；
- seed 只补不覆盖，线上配置以本模块保存为准，两者共存；
- **任何写操作后必须调 `InvalidateAllPermissions`** 全量失效权限缓存（跨所有租户，无法按租户定位）；
- 删除节点前处理其子节点与角色引用关系（软删）。

## DTO

`Node`（树节点，含 children）、`Input`（新建/编辑入参）。
