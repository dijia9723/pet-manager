# internal/middleware/rbac_test.go

> 脚手架导航：[回到开发者入口](../../../README.md) · [rbac.md](rbac.md)

## 职责

不依赖外部服务的纯函数单测，覆盖 `HasAny` 的判定矩阵：

- 无 required 时恒真（登录即可）；
- 持有通配 `*` 恒真（平台管理员）；
- 精确命中 / 全部不命中；
- 空集合不误放。

真实权限解析、缓存命中/回源、403+20005 链路在 [集成测试](../tests/integration_test.md) 验证。
