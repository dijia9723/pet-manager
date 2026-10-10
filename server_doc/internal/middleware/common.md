# internal/middleware/common.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [registry.md](registry.md)

## 职责

通用全局中间件集合（序位 10~60），全部注册在 [bootstrap](../bootstrap/app.md)。

| 中间件 | 序位 | 说明 |
| --- | --- | --- |
| `Recovery(log)` | 10 | panic 兜底，打全栈日志后返回 10000，保证不打死进程 |
| `RequestID()` | 20 | 沿用入站 `X-Request-Id` 或生成随机 token；写 context 与响应头 |
| `SecurityHeaders()` | 30 | `X-Content-Type-Options` / `X-Frame-Options: DENY` / `Referrer-Policy` |
| `CORS(cfg)` | 40 | 白名单精确匹配 Origin；OPTIONS 预检直接 204；非白名单不放行 |
| `RateLimit(rdb, 600, 1m)` | 50 | 按 IP 的 Redis 固定窗口；**Redis 故障 fail-open**，不阻断业务 |
| `AccessLog(log)` | 60 | 结构化访问日志，带 request_id/租户/账号；≥500 error、≥400 warn |

## 约定

- 新的通用中间件先在 [registry.md](registry.md) 的序位表选档位，再 `Add` 注册；
- 访问日志的租户/账号字段取自 context，因此组级鉴权中间件必须在 c.Next 前完成写入；
- 限流超限返回业务码 `10004`（HTTP 429）。
