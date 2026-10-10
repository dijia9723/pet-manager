# curl 解析

> [开发者入口](../../../../README.md) · [任务管理](service.md) · [执行器](executor.md) · [模型](../../model/job.md)

## 职责

把任务配置里的 curl 文本解析为 `ExecutionConfig`，随后交由 Go HTTP 客户端执行。curl **命令本身不会被送入 Shell**，因此不存在命令注入面；它只是被当作 HTTP 请求语法解析。

## 关键函数

| 函数 | 说明 |
| --- | --- |
| `parseCurl(command string)` | 分词后逐项识别，产出 `ExecutionConfig`。首个词必须是 `curl`/`curl.exe`；只允许一个请求地址 |
| `curlWords(command string)` | 按 POSIX 习惯做分词：支持单/双引号、反斜杠转义与反斜杠续行；拒绝对多字节不安全的空字符 |

## 支持的语法

- 方法：`-X/--request`、`-I/--head`、`-G/--get`；
- 请求头：`-H/--header`（同名头会以逗号合并，键名做 `CanonicalHeaderKey` 归一）；
- 请求体：`-d/--data`、`--data-raw`、`--data-binary`、`--data-urlencode`、`--json`（自动补 `Content-Type`/`Accept`）；
- 认证：`-u/--user`（转 Basic Auth）、`-b/--cookie`；
- 其它：`--url`、静默/错误/重定向类开关（`-s -S -L --compressed` 等）。

## 约定与坑

- **不接受**文件读取（`@file`，`--data-raw` 例外，按字面量处理）、管道、重定向、命令串联、环境变量展开；命中 `; | & < > \`` 直接报错。
- `-G` 会把 `--data*` 内容拼到 URL query；`--data-urlencode` 会做 query 转义。
- 无显式方法时：有请求体按 `POST`（缺省 `application/x-www-form-urlencoded`），否则 `GET`。
- 解析结束统一走 `validateHTTP`（仅 http/https、禁止 URL 内嵌账号、限制方法与头字段）。
- 复杂请求建议改用请求表单，不要绕过校验。
