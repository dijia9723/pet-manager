# internal/model/login_log.go — sys_login_log

> 脚手架导航：[回到开发者入口](../../../README.md) · [loginlog 模块](../modules/loginlog/service.md) · [auth 模块](../modules/auth/handler.md)

登录日志表（平台公共库），成功与失败尝试都记录（安全价值主要在失败侧），`tenant_id=0` 表示平台账号。

字段：username / user_type / tenant_id / tenant_code / client_ip / user_agent / status(`success|fail`) / message + BaseModel。

- `status` 用业务语义字符串，与 [audit_log](audit_log.md) 的 HTTP 码 int 刻意区分；
- 复合索引（migrate 补建）：`(tenant_id, created_at DESC)`、`(username, created_at DESC)`；
- 由 auth 模块登录流程内 best-effort 采集，写失败不影响登录响应。
