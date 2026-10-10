# internal/modules/audit/handler.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [module.md](module.md) · [service.md](service.md) · Swagger 分组「平台-审计日志」

## 职责

审计日志只读查询 HTTP 层，唯一接口：

| 方法路径 | 权限点 | 入参 |
| --- | --- | --- |
| GET `/platform/audit-logs` | `platform.audit.list` | ListQuery（tenant_id/user_id/username/method/path/status/start/end + 分页） |

时间边界支持三种格式（RFC3339、`2006-01-02 15:04:05`、`2006-01-02`），只给日期时结束边界取次日零点覆盖整天。采集侧不经过这里，见 [service.md](service.md)。
