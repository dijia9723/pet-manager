# internal/pkg/pagination/pagination.go

> 脚手架导航：[回到开发者入口](../../../../README.md)

## 职责

统一所有列表接口的分页入参，被各模块的 `ListQuery` 匿名嵌入。

```go
type ListQuery struct {
    pagination.Query
    Keyword string `form:"keyword"`
}
```

## 规则

- 入参字段：`page` / `page_size`（form tag）；
- 默认页码 1，默认每页 20，**上限 200**（防止恶意拉全表）；
- `Current()` / `Limit()` / `Offset()` 输出规范化后的值，service 层直接用，不自行判空。

返回结构统一用 [response.Page[T]](../response/response.md)（`{list,total}`）。
