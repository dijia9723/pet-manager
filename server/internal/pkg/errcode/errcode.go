// Package errcode 定义全站错误码与业务错误类型。
// 约定：
//   - 0 表示成功；
//   - 1xxxx 为通用/框架级错误；
//   - 2xxxx 为认证与租户域错误；
//   - 3xxxx 为业务域错误（后续模块自行扩展）。
package errcode

import (
	"fmt"
	"net/http"
)

// Code 是业务错误码。它独立于 HTTP 状态码：同一 Code 经 HTTPStatus 映射到
// 合适的 HTTP 语义，前端则统一读响应体中的 code 做精细提示。
type Code int

// 通用与框架级错误码（1xxxx），供任意模块复用。
const (
	OK Code = 0 // 成功

	ErrInternal           Code = 10000 // 未归类的内部错误
	ErrInvalidParam       Code = 10001 // 参数校验失败
	ErrNotFound           Code = 10002 // 资源不存在
	ErrConflict           Code = 10003 // 资源冲突
	ErrTooManyRequests    Code = 10004 // 触发限流
	ErrNotImplemented     Code = 10005 // 尚未实现
	ErrDatabase           Code = 10006 // 持久层错误
	ErrUnauthorized       Code = 10401 // 未认证
	ErrForbidden          Code = 10403 // 无权限
	ErrRequestTooLarge    Code = 10413 // 请求体超限
	ErrServiceUnavailable Code = 10503 // 依赖不可用

	// 认证 / 租户域
	ErrLoginFailed      Code = 20001
	ErrTokenInvalid     Code = 20002
	ErrTokenExpired     Code = 20003
	ErrSessionRevoked   Code = 20004
	ErrForbiddenAction  Code = 20005
	ErrTenantNotFound   Code = 20006
	ErrTenantDisabled   Code = 20007
	ErrTenantExpired    Code = 20008
	ErrTenantSchemaBad  Code = 20009
	ErrTenantCodeTaken  Code = 20010
	ErrUsernameTaken    Code = 20011
	ErrAccountDisabled  Code = 20012
	ErrCannotDeleteSelf Code = 20013
	ErrOwnerProtected   Code = 20014
	ErrRoleInUse        Code = 20015
	ErrRoleCodeTaken    Code = 20016
	ErrSystemRoleLocked Code = 20017
	ErrCrossTenant      Code = 20018
)

var messages = map[Code]string{
	OK: "ok",

	ErrInternal:           "服务器内部错误",
	ErrInvalidParam:       "请求参数不合法",
	ErrNotFound:           "资源不存在",
	ErrConflict:           "资源冲突",
	ErrTooManyRequests:    "请求过于频繁，请稍后再试",
	ErrNotImplemented:     "功能尚未实现",
	ErrDatabase:           "数据库操作失败",
	ErrUnauthorized:       "未登录或登录状态已失效",
	ErrForbidden:          "无权限访问",
	ErrRequestTooLarge:    "请求体过大",
	ErrServiceUnavailable: "服务暂时不可用",

	ErrLoginFailed:      "用户名或密码错误",
	ErrTokenInvalid:     "登录凭证无效",
	ErrTokenExpired:     "登录凭证已过期",
	ErrSessionRevoked:   "登录状态已被注销",
	ErrForbiddenAction:  "无权限执行该操作",
	ErrTenantNotFound:   "租户不存在",
	ErrTenantDisabled:   "租户已被停用",
	ErrTenantExpired:    "租户已到期",
	ErrTenantSchemaBad:  "租户数据空间标识非法",
	ErrTenantCodeTaken:  "租户编码已被占用",
	ErrUsernameTaken:    "账号已存在",
	ErrAccountDisabled:  "账号已被停用",
	ErrCannotDeleteSelf: "不能删除当前登录账号",
	ErrOwnerProtected:   "租户主账号不允许被删除或停用",
	ErrRoleInUse:        "角色已被账号引用，无法删除",
	ErrRoleCodeTaken:    "角色编码已存在",
	ErrSystemRoleLocked: "预置角色不允许修改编码或删除",
	ErrCrossTenant:      "跨租户访问被拒绝",
}

// Message 返回错误码的默认中文文案。
func (c Code) Message() string {
	if m, ok := messages[c]; ok {
		return m
	}
	return fmt.Sprintf("未知错误(%d)", int(c))
}

// HTTPStatus 把业务错误码映射为 HTTP 状态码，保证前端既能按 HTTP 语义处理，
// 也能按统一响应体里的 code 做精细化提示。
func (c Code) HTTPStatus() int {
	switch c {
	case OK:
		return http.StatusOK
	case ErrInvalidParam:
		return http.StatusBadRequest
	case ErrUnauthorized, ErrLoginFailed, ErrTokenInvalid, ErrTokenExpired, ErrSessionRevoked:
		return http.StatusUnauthorized
	case ErrForbidden, ErrForbiddenAction, ErrCrossTenant:
		return http.StatusForbidden
	case ErrNotFound, ErrTenantNotFound, ErrTenantSchemaBad:
		return http.StatusNotFound
	case ErrConflict, ErrTenantCodeTaken, ErrUsernameTaken, ErrRoleCodeTaken:
		return http.StatusConflict
	case ErrTenantDisabled, ErrTenantExpired, ErrAccountDisabled:
		return http.StatusForbidden
	case ErrTooManyRequests:
		return http.StatusTooManyRequests
	case ErrRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	case ErrNotImplemented:
		return http.StatusNotImplemented
	case ErrServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Error 是贯穿 handler -> service -> repo 的统一业务错误。
type Error struct {
	Code  Code
	Msg   string
	cause error
}

// New 用错误码的默认文案构造业务错误。
func New(code Code) *Error {
	return &Error{Code: code, Msg: code.Message()}
}

// Newf 以错误码默认文案之外的自定义格式构造业务错误（如拼接具体字段名）。
func Newf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

// Wrap 在保留业务码的同时携带底层原因，便于日志追溯。
func Wrap(code Code, cause error) *Error {
	return &Error{Code: code, Msg: code.Message(), cause: cause}
}

// Error 实现 error 接口，输出形如 "[20001] 用户名或密码错误: <cause>"。
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Msg, e.cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg)
}

// Unwrap 暴露底层原因，配合 errors.Is / errors.As 做错误链判断。
func (e *Error) Unwrap() error { return e.cause }

// From 把任意 error 归一化为 *Error，非业务错误统一按 ErrInternal 处理。
func From(err error) *Error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*Error); ok {
		return e
	}
	return Wrap(ErrInternal, err)
}
