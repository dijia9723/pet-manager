# internal/modules/tenant/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [handler.md](handler.md) · [migrate](../../migrate/migrate.md)

## 职责

平台侧租户管理 + 给鉴权链提供租户快照（实现 `middleware.TenantResolver`）。

## 能力

`TenantByID`（带 Redis 快照缓存）、`List`、`Detail`、`Create`、`Update`、`SetStatus`、`Delete`、`Restore`、`RunPurge`；立即销毁 `Destroy` 和共享销毁编排在 [purge.md](purge.md)。

## 开通编排（Create → provision，硬约束）

同一事务内原子完成四步，任一步失败整体回滚：

1. 建 `sys_tenant` 记录（Code 存活唯一校验，SchemaName 由 ID 推导）；
2. `migrate.TenantSchema` 建 schema + 全量租户表 + 预置数据；
3. 预置 owner/staff 角色；
4. 建主账号（`sys_tenant_user`，is_owner=true，bcrypt 口令）。

事务外另有 Redis 分布式锁串行化开通、`compensate` 做失败补偿（删除残留 schema/记录）。

## 两段式删除与到期销毁

1. **`Delete(ctx, id)`（请求路径，软删/待销毁）**
   - 租户置 `status=deleted`、`purge_at=now+N 天`（N 取平台参数 `tenant.purge_after_days`，缺省/停用/非法回退默认 7，见 `purgeGraceDays`），写库即生效、不做物理删除，接口幂等（重复删除只刷新宽限期）；
   - 随后 `RevokeTenantSessions` 踢该租户全部会话 + `InvalidateTenant` 失效快照；租户门禁中间件对非 enabled 状态直接拦截，待销毁租户无法登录、无法调接口；
   - code/schema_name 的存活唯一索引不受影响（行仍在），宽限期内不可用同 code 重开。
2. **`Restore(ctx, id)`（撤销删除）**
   - 仅 `deleted` 可恢复：置回 `disabled` 并清空 purge_at（安全口径，恢复后需手动启用）；非 deleted 返回 `ErrConflict`（409）。
3. **`RunPurge(ctx) (string, error)`（内置任务 tenant_purge 的执行体，每日 03:30）**
   - 扫描 `status='deleted' AND purge_at <= now()`，逐个调用 `purgeOne`，行锁下重新检查状态和到期时间；已恢复或已被立即销毁的记录跳过。
   - 每个租户的 schema 与平台归属记录在同一 PostgreSQL 事务中销毁，失败整体回滚；已完成其他租户不会回滚，下一轮重试剩余。成功返回摘要"已销毁 N 个租户：code…"；
   - 附件和存储对象标记为后台待清理，在途上传等待完成或超时，保留台账与占用直到对象删除成功；
   - 审计日志、登录日志保留在平台库（合规留痕），不随租户数据销毁；
   - 可在「任务管理」页手动执行用于运维（宽限天数设 0 即立即到期）。

## 其他约定

- `SetStatus` 停用后 `RevokeTenantSessions` + 失效快照缓存，拦截立即生效；
- 资料变更走 `invalidate` 清租户快照；
- `Update` 更新名称、编码、联系方式，不改 SchemaName/状态。使用限定 ID 的 `Updates`，并发销毁导致影响行数为 0 时返回不存在，避免 `Save` 的回退插入复活租户；
- 平台连接直接查 `t_role` 等租户表属违规，跨库操作一律在开通/迁移事务内完成。
