# 执行器

> [开发者入口](../../../../README.md) · [任务管理](service.md) · [curl 解析](curl.md) · [Cron](cron.md)

## 职责

定义任务的执行体抽象与三类执行器（system / http / shell），负责配置校验、方法注册与执行。

## 关键类型

| 类型 | 说明 |
| --- | --- |
| `TaskFunc` | `func(context.Context) (string, error)`，任务执行体；返回值写入日志 |
| `Method` / `MethodCatalog` | 可被任务页选择的系统方法及方法目录返回体（含 `ShellAvailable`） |
| `ExecutionConfig` | 三类任务共用的执行配置（`Handler`/URL 表单/`Curl`/脚本与参数等，仅当前类型字段生效） |
| `ExecutorOptions` | 执行器可选配置，目前仅 `ShellPath` |
| `boundedOutput` | 并发安全的限长缓冲，脚本 stdout/stderr 共用 |

## 关键函数

- `RegisterMethod`：注册系统方法，**必须在调度器启动前**完成；缺字段、编码重复或启动后注册直接 panic。
- `Methods`：返回按编码排序的方法目录，并 `LookPath` 探测解释器是否可用。
- `normalizeConfig`：按类型校验并规范化配置（system 校验方法已注册；http 解析 curl 或表单并 `validateHTTP`；shell 校验脚本/路径/参数边界与解释器存在）。
- `validateHTTP`：仅 http/https、拒绝 URL 内嵌账号、白名单方法、请求头名称/内容校验，`Content-Length`/`Transfer-Encoding` 由系统设置。
- `task`：把任务行解析为执行体与“调度身份”（system 用方法名，http/shell 用 `job:编码`），身份用于跨实例互斥。
- `invoke`：执行体的 panic 兜底转错误；`ctx` 已取消时以 `ctx.Err()` 为准。
- `runHTTP`：发起请求，最多 5 次重定向且重定向协议受限，读取体量受限，非 2xx 记失败。
- `runShell`：脚本内容经 **stdin** 送入（不落 argv），文件脚本按路径执行；`configureProcess` 负责超时/取消时终止进程组。

## 约定与坑

- 单次输出/请求体/脚本统一上限 `maxOutputBytes` = 64 KiB；输出超限截断并按 rune 边界切分，非法 UTF-8 替换为 `�`。
- 任务编码规则 `jobCodePattern`：字母开头，仅字母数字与 `_ . -`，≤64 位。
- 旧版预置任务 `config` 为空时，system 类型回退以任务编码作为方法名（`decodeConfig`），兼容历史数据。
