# internal/modules/audit/module.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [handler.md](handler.md) · [service.md](service.md)

## 职责

审计模块的路由注册，模块名 `platform.audit`。

```go
g := r.Platform("/audit-logs")
g.GET("", r.Require("platform.audit.list"), s.handleList)
```

只读模块：平台侧仅暴露分页查询，不提供新建/编辑/删除接口（审计记录只能由中间件采集产生）。
