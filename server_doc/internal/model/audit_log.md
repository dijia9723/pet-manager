# internal/model/audit_log.go — sys_audit_log

> 脚手架导航：[回到开发者入口](../../../README.md) · [审计中间件](../middleware/audit.md) · [audit 模块](../modules/audit/service.md)

审计日志表（平台公共库），一张表同时容纳平台与租户的写操作事件：`tenant_id=0` 表示平台事件。

字段：tenant_id / user_type / user_id / username / method / path / route / status(HTTP 码) / client_ip / user_agent / latency_ms / request_id + BaseModel。

## 设计取舍

- **不记录请求体/响应体**——明文口令绝不入审计表；追溯靠 route+status+操作人+request_id 回查访问日志；
- 复合索引（migrate 补建）：`(tenant_id, created_at DESC)`、`(user_id, created_at DESC)`；
- 写入路径为非阻塞队列，队列满丢弃计数，绝不拖慢主请求。
