# internal/modules/audit/policy_test.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [policy.go](policy.go.md)

覆盖采集策略判定（不依赖 DB）：默认策略只记写方法、总开关关闭全不记、自定义方法集（含 GET）、排除路由精确/通配/边界、`parseMethods` 的 trim/大写/去重与空输入。
