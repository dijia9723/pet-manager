# internal/infra/session/store.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [cache/redis.md](../cache/redis.md) · [middleware/auth.md](../../middleware/auth.md)

## 职责

统一管理三类生命周期高度相关的 Redis 数据：

| 数据 | key 形态 | TTL | 失效时机 |
| --- | --- | --- | --- |
| 会话 | `pm:session:{userType}:{uid}:{sid}` | refresh 有效期 | 登出 / 踢下线 / 改密停用 |
| 权限点 | `pm:perm:t:{tenantID}:{userID}` | 10min | 角色授权变更（按租户）/ 权限点字典变更（全量） |
| 租户快照 | `pm:tenant:{id}` | 5min | 停用/续费/资料修改 |

## 设计要点

- access 与 refresh 共用一个 `sid`，注销即删 key——JWT 无状态与"可强制下线"的折中；
- `RevokeUserSessions` 按 `session:{userType}:{uid}:` 前缀清理，保留末尾分隔符，避免 UID 1 误匹配 UID 10，改密/停用/删除账号与租户销毁时调用；
- `TenantInfo.IsExpired`：`ExpireAt` 为空视为永久有效；
- 权限缓存两级失效策略：
  - `InvalidateTenantPermissions`：角色/授权变更和租户销毁，只清 `perm:t:{tenantID}:`，保留末尾分隔符，避免租户 ID 前缀重叠误清其他租户；
  - `InvalidateAllPermissions`：权限点字典增删改，跨租户整前缀清理。

## 注意

- 缓存反序列化失败按"未命中"处理，自动回源重建，不把脏缓存升级成错误；
- 快照中的 `schema` 会在 [TenantGuard.Context](../../middleware/tenant.md) 中重新过正则校验并以数据库为准回填。
