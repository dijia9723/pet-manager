// Package response 统一 HTTP 响应结构与错误出口。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"pet-manager/server/internal/pkg/errcode"
)

// Body 是全站唯一响应体。前端只需判断 code == 0。
type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Page 是分页数据的统一结构。
type Page[T any] struct {
	List  []T   `json:"list"`
	Total int64 `json:"total"`
}

// NewPage 构造分页响应；list 为 nil 时归一化为空数组，
// 保证前端拿到的始终是 [] 而非 null。
func NewPage[T any](list []T, total int64) *Page[T] {
	if list == nil {
		list = []T{}
	}
	return &Page[T]{List: list, Total: total}
}

// OK 返回成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: int(errcode.OK), Message: errcode.OK.Message(), Data: data})
}

// Fail 返回业务错误响应，HTTP 状态码由错误码映射得出。
func Fail(c *gin.Context, err error) {
	e := errcode.From(err)
	c.JSON(e.Code.HTTPStatus(), Body{Code: int(e.Code), Message: e.Msg})
}

// Abort 用于中间件：写入响应并终止后续 handler。
func Abort(c *gin.Context, err error) {
	e := errcode.From(err)
	c.AbortWithStatusJSON(e.Code.HTTPStatus(), Body{Code: int(e.Code), Message: e.Msg})
}
