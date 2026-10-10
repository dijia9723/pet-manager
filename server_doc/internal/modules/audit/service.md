# internal/modules/audit/service.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [审计中间件](../../middleware/audit.md) · [audit_log 模型](../../model/audit_log.md)

## 职责

审计日志的**采集落库**与**平台查询**两端，模块名 `platform.audit`。

## 采集侧（实现 middleware.AuditRecorder）

- 缓冲队列 `queueSize=1024` + 单 worker；
- `Submit` 非阻塞：队列满直接丢弃并计数，每丢满 1000 条告警一次（不刷屏）；
- `write` 单条 3s 超时，失败只 warn；审计故障绝不影响主请求；
- `ShouldRecord` 读 `atomic.Pointer[policy]` 快照做采集决策（总开关 / 方法白名单 / 排除路由），策略来源与热更新见 [policy.md](policy.md)；
- `Run` 幂等（atomic CAS）：先同步回源一次策略，再起 worker；worker 同 goroutine 内每 `policyTTL=60s` 刷新策略（低频且有 3s 超时，不单独开 goroutine），加载失败保留旧快照；`Close` 发停止信号并等待 `drain` 把队列剩余记录 flush 完（优雅退出时调用）。

## 查询侧

`List(ctx, ListQuery)` 查平台库，按 id 倒序分页，支持租户/账号/用户名/方法/路径/状态/时间区间组合筛选，返回 `response.Page[Item]`。

依赖：db.Manager（仅平台库）、zap.Logger。
