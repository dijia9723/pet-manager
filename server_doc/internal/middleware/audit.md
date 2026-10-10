# internal/middleware/audit.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [audit 模块](../modules/audit/service.md)

## 职责

审计采集中间件（全局序位 900），与落库实现解耦。

## 采集规则（硬约定）

- **未登录请求一律不记**：登录、刷新、公开路由天然不记（UserType 为空直接跳过）；
- **是否记录由策略决定**：中间件不再内置方法白名单，放行后调用 `AuditRecorder.ShouldRecord(method, route)` 判定。默认策略等价于历史行为——只记 `POST/PUT/PATCH/DELETE`，策略由参数配置（sys_config 的 `audit` 分组）治理、约 60 秒热生效，详见 [audit/policy.md](../modules/audit/policy.md)；
- 判定用的路由优先取 `c.FullPath()` 模板（形如 `/api/v1/platform/tenants/:id`，不含具体 id），404 等未命中场景回退实际路径；
- **不记录请求体/响应体**：重置密码等接口的明文口令绝不能进审计表；
- 追溯方式：route + status + 操作人 + request_id 回查访问日志；
- 先 `c.Next()` 放行，等鉴权链与业务执行完再读最终 user_type/tenant_id/status，因此拿到的是终态。

## 投递方式

`AuditRecorder` 接口由 [audit.Service](../modules/audit/service.md) 实现：`ShouldRecord` 读原子策略快照（零锁零 DB），`Submit` 为**非阻塞投递**（缓冲队列 + 单 worker，队列满丢弃计数），审计失败绝不影响主请求。

`AuditEntry` 是中间件与模块间的传输结构，字段与 [model.AuditLog](../model/audit_log.md) 对齐但不耦合 GORM。
