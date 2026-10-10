# internal/middleware/auth.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [jwtx](../pkg/jwtx/jwt.md) · [ctxkey](../pkg/ctxkey/ctxkey.md)

## 职责

鉴权链路第一环：**凭证解析 + 会话校验**。

`Authenticator.Require(userType)`：

1. 从 `Authorization: Bearer <token>` 取令牌。浏览器 WebSocket 升级请求也可通过 `Sec-WebSocket-Protocol` 中的 `bearer.<token>` 携带令牌，避免进入 URL；仅 `Upgrade: websocket` 时启用，Authorization 优先。保留 `?access_token=` 预留场景；
2. `jwtx.Manager.Parse` 验签、校验 issuer 与 HS256，要求 `typ=access`；
3. 如传入期望身份（`platform` / `tenant`），不匹配直接 403（业务码 20005）；
4. 查 Redis 会话是否仍存活（登出/踢下线可立即生效）；
5. 把 user_id/username/user_type/session_id 写入 context，租户身份额外写 tenant_id/code/schema。

## 依赖反转

中间件只依赖接口，不依赖 modules：

- `PermissionResolver`（rbac 模块实现）；
- `TenantResolver`（tenant 模块实现，返回带缓存的租户快照）。

## 相关错误码

`10401` 未登录 · `20002` 凭证无效 · `20003` 凭证过期 · `20004` 会话已注销 · `20005` 身份/权限不足。

辅助函数 `CurrentUserID(c)` 与 `ctxkey.UserID(ctx)` 等价。
