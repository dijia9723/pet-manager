# internal/modules/loginlog/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [sys_login_log 模型](../../model/login_log.md) · [auth 模块](../auth/service.md)

## 职责

登录日志的记录与查询，模块名 `platform.loginlog`。

- **记录侧**：提供登录事件采集方法（实现 `auth.LoginRecorder`），由 auth 登录流程 best-effort 调用，写库失败只记日志不影响登录；
- **查询侧**：`List(ctx, ListQuery)` 平台库分页，按时间倒序，支持用户名/租户/结果（success/fail）/时间区间筛选，返回 `response.Page[Item]`。

依赖：db.Manager（仅平台库）、zap.Logger。字段口径见 [login_log 模型](../../model/login_log.md)。
