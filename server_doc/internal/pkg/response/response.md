# internal/pkg/response/response.go

> 脚手架导航：[回到开发者入口](../../../../README.md) · [errcode](../errcode/errcode.md) · Swagger 中所有接口的 data 均包在此结构内

## 职责

全站唯一 HTTP 响应出口。前端只需判断 `code == 0`。

```go
type Body struct {
    Code    int    `json:"code"`     // 0 成功，其余见 errcode
    Message string `json:"message"`
    Data    any    `json:"data,omitempty"`
}

type Page[T any] struct {
    List  []T   `json:"list"`
    Total int64 `json:"total"`
}
```

## 三个出口函数

| 函数 | 使用位置 | 行为 |
| --- | --- | --- |
| `OK(c, data)` | handler 成功 | HTTP 200 + code=0 |
| `Fail(c, err)` | handler 失败 | HTTP 状态由 `errcode.Code.HTTPStatus()` 映射 |
| `Abort(c, err)` | 中间件失败 | 写响应并 `Abort` 终止链路 |

## 约定

- handler 只允许通过这三个函数写 JSON，不直接 `c.JSON`（探活等极少数场景除外）；
- 分页数据用 `response.NewPage(list, total)`，nil 列表会被规范成 `[]`；
- Swagger 注解统一写 `@Success 200 {object} response.Body`，data 的实际类型写在描述里。
