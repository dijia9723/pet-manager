# internal/pkg/errcode/errcode.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [response](../response/response.md)

## 职责

全站错误码字典 + 统一业务错误类型，贯穿 handler → service → repo。

## 号段约定

| 号段 | 用途 |
| --- | --- |
| `0` | 成功 |
| `1xxxx` | 通用/框架级（10000 内部错误、10001 参数、10002 不存在、10004 限流…） |
| `104xx/105xx` | 对齐 HTTP 语义的通用码（10401/10403/10413/10503） |
| `2xxxx` | 认证与租户域（20001 登录失败、20004 会话注销、20005 无权限、20006~20010 租户相关、20011~20018 账号/角色相关） |
| `3xxxx` | 预留给后续业务域 |

## 用法

```go
errcode.New(errcode.ErrInvalidParam)                 // 默认文案
errcode.Newf(errcode.ErrInvalidParam, "字段 %s 非法", f) // 自定义文案
errcode.Wrap(errcode.ErrDatabase, err)               // 保留 cause，日志可追溯
errcode.From(err)                                    // 非业务错误统一归 10000
```

`Code.HTTPStatus()` 维护业务码 → HTTP 状态码映射：前端既可按 HTTP 语义处理，也能按响应体 `code` 精细化提示。**新增错误码时两处都要加**（常量 + messages 文案，必要时加 HTTP 映射）。
