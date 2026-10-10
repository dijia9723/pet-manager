# internal/infra/logger/logger.go

> 脚手架导航：[回到开发者入口](../../../../README.md)

## 职责

基于 zap 构建全局日志器。

- `format=console`（默认）：带颜色的控制台输出，本地开发；
- `format=json`：JSON 结构日志，供容器日志采集；
- 时间字段 `ts`，ISO8601 格式；级别大写；
- 始终开启 `AddCaller`，Error 及以上带堆栈；
- sink 仅支持 stdout/stderr，落盘交给容器侧采集，不自己切分文件。

## 使用

- 构造一次，在 `bootstrap.New` 注入各 Service / 中间件，不要在业务代码里 `zap.New`；
- 无日志器上下文时不要用全局 `zap.L()`（未初始化时是空实现），应显式传参。
