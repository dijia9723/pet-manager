# internal/modules/dict/tenant_service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [t_dict](../../model/tenant_dict.md) · [t_dict_item](../../model/tenant_dict_item.md) · 平台侧 [service.md](service.md) · [tenant_handler.md](tenant_handler.md)

## 职责

租户字典类型与字典项（`t_dict` / `t_dict_item`）的全部业务，与平台侧共用 `Item/Entry/*Input` 契约。

## 取数入口（硬约定）

- 所有方法第一步都调 `ctxkey.MustTenantDB(ctx)` 取请求级租户事务句柄；
- 该句柄由 TenantScope 中间件注入，`search_path` 已设为 `"tenant_x, public"`，租户之间互不可见；
- 拿不到句柄属于编码错误，直接报错，**禁止回退平台连接**。

## 规则

- 与平台侧一致：字典 Code 存活唯一；字典项 `(dict_id, value)` 存活唯一；
- 删除字典类型时在同一事务内级联软删其下字典项（`DeleteTenantDict`）；
- 字典项列表按 `sort asc, id asc` 返回全量（不分页）；
- 唯一性校验/字典加载等内部函数（`loadTenantDict` 等）接收外部传入的 `*gorm.DB`，保证运行在当前租户事务内。
