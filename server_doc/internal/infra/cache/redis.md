# internal/infra/cache/redis.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [session/store.md](../session/store.md)

## 职责

封装 go-redis 客户端，统一 key 前缀（默认 `pm:`），避免各模块各写一套。

## API 速查

| 方法 | 用途 |
| --- | --- |
| `New(cfg)` | 建连并 Ping（5s 超时），失败直接启动失败 |
| `Key(parts...)` | 统一拼前缀 key，如 `pm:session:tenant:42:xxx` |
| `Get/Set/Del` | 基础读写，Set 带 TTL |
| `DelByPrefix` | SCAN 批量删除（权限/会话失效用），游标分批每批 200 |
| `AcquireLock/ReleaseLock` | SetNX 简易分布式锁（租户开通串行化用） |
| `Raw()` | 需要原生命令（Exists/Incr/Expire/Scan）时使用，限流器即用它 |
| `Close()` | 优雅退出时关连接 |

## 使用方

- [session.Store](../session/store.md)：会话 / 权限点 / 租户快照；
- [middleware.RateLimit](../../middleware/common.md)：IP 固定窗口限流；
- 租户开通的分布式锁（tenant service）。
