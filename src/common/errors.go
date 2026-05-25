package common

import (
	"fmt"
	"net/http"
)

// AppError 统一应用错误类型
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewError 创建应用错误
func NewError(code int, message string, err ...error) *AppError {
	ae := &AppError{Code: code, Message: message}
	if len(err) > 0 {
		ae.Err = err[0]
	}
	return ae
}

// 预定义常用错误
var (
	ErrNotFound     = NewError(http.StatusNotFound, "资源不存在")
	ErrBadRequest   = NewError(http.StatusBadRequest, "请求参数错误")
	ErrUnauthorized = NewError(http.StatusUnauthorized, "未授权访问")
	ErrForbidden    = NewError(http.StatusForbidden, "禁止访问")
	ErrInternal     = NewError(http.StatusInternalServerError, "服务器内部错误")
)

// HTTPStatusText 根据状态码返回中文描述
func HTTPStatusText(code int) string {
	texts := map[int]string{
		400: "请求参数错误",
		401: "未授权访问",
		403: "禁止访问",
		404: "页面不存在",
		405: "请求方法不允许",
		500: "服务器内部错误",
		502: "网关错误",
		503: "服务不可用",
	}
	if text, ok := texts[code]; ok {
		return text
	}
	return http.StatusText(code)
}
