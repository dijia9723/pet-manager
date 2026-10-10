# internal/modules/tenant/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [service.md](service.md) · Swagger 分组「平台-租户管理」

## 职责

平台端租户管理 HTTP 层，挂 `Platform("/tenants")`，全部接口需平台身份 + 对应权限点。

| 方法路径 | 权限点 | 说明 |
| --- | --- | --- |
| GET `/platform/tenants` | `platform.tenant.list` | 分页（keyword/status） |
| POST `/platform/tenants` | `platform.tenant.create` | body: CreateInput（含主账号） |
| GET `/platform/tenants/:id` | `platform.tenant.list` | 详情 |
| PUT `/platform/tenants/:id` | `platform.tenant.update` | body: UpdateInput |
| POST `/platform/tenants/:id/enable` | `platform.tenant.update` | 启用 |
| POST `/platform/tenants/:id/disable` | `platform.tenant.update` | 停用（会话全失效） |
| POST `/platform/tenants/:id/delete` | `platform.tenant.delete` | 两段式删除：进入 deleted 待销毁态 + 写 purge_at + 踢会话（幂等） |
| POST `/platform/tenants/:id/restore` | `platform.tenant.delete` | 撤销删除：deleted → disabled 并清 purge_at；非待销毁态返 409 |
| POST `/platform/tenants/:id/destroy` | `platform.tenant.destroy` | body: DestroyInput，confirm_code 精确匹配后跳过宽限期立即物理销毁；返回 DestroyResult.pending_resources |

`:id` 统一由本文件 `pathID` 解析（非零 uint64，非法返 10001）。立即销毁适用于启用、停用、待销毁状态，仅平台专属权限可调用；错误编码拒绝销毁，重复销毁返回不存在。定时到期销毁与立即销毁共享 [purge.md](purge.md) 的事务编排；见 [service.md](service.md) 与 [任务模块](../job/service.md)。
