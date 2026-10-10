# 执行器测试

> [开发者入口](../../../../README.md) · [执行器](executor.md) · [任务管理](service.md)

覆盖 `executor.go` 与 `curl.go` 的执行与解析逻辑，均为本地用例（不起真实调度）。

## 覆盖范围

- **curl 语法解析**（`TestCurlRequests`）：合法写法（引号、续行、`-G --data-urlencode`、`--json`、HEAD 等）可解析，且管道/重定向/文件读取等被拒绝。
- **HTTP 执行**：通过 `httptest` 校验方法、请求头、请求体与重定向、非 2xx 判定、上下文取消。
- **shell 执行**：真实调用 `sh`，覆盖脚本内容经 stdin 传入、脚本参数、非零退出、超时与子进程终止。
- **配置归一化**：类型校验与边界（脚本/路径/参数、URL 与方法白名单）。

## 约定

用例不依赖外部 PostgreSQL/Redis；需要真实库的集成场景见 [集成测试](integration_test.md)。
