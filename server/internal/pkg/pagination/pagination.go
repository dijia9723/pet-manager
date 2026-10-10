// Package pagination 统一列表接口的分页参数解析。
package pagination

const (
	defaultPage     = 1   // 未传页号时的起始页
	defaultPageSize = 20  // 未传每页条数时的默认值
	maxPageSize     = 200 // 每页条数上限，防止一次拉取过多数据
)

// Query 是列表接口通用的分页查询参数，直接由 gin 绑定 query/form。
// 零值（未传参）也能得到合理的默认分页，因此无需在 handler 里判空。
type Query struct {
	Page     int `form:"page"`      // 页码，从 1 开始
	PageSize int `form:"page_size"` // 每页条数
}

// Limit 返回规范化后的每页条数。
func (q Query) Limit() int {
	if q.PageSize <= 0 {
		return defaultPageSize
	}
	if q.PageSize > maxPageSize {
		return maxPageSize
	}
	return q.PageSize
}

// Offset 返回偏移量。
func (q Query) Offset() int {
	page := q.Page
	if page <= 0 {
		page = defaultPage
	}
	return (page - 1) * q.Limit()
}

// Current 返回规范化后的页码。
func (q Query) Current() int {
	if q.Page <= 0 {
		return defaultPage
	}
	return q.Page
}
