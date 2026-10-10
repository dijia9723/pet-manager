# internal/modules/loginlog/module.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [handler.md](handler.md) · [service.md](service.md)

## 职责

登录日志模块路由注册，模块名 `platform.loginlog`。挂 `Platform("/login-logs")`，只有一个只读列表接口（`platform.loginlog.list`），刻意不暴露任何写/删接口。

本模块 Service 在 [bootstrap](../../bootstrap/app.md) 中先于 auth 构造，作为 `auth.LoginRecorder` 注入登录流程。
