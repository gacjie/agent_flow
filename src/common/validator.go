package common

import (
	"fmt"
	"net/http"
	"strings"

	"agent_flow/src/config"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// GetValidator 获取校验器实例
func GetValidator() *validator.Validate {
	return validate
}

// ValidateStruct 校验结构体，返回格式化的错误信息
func ValidateStruct(s interface{}) error {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	messages := make([]string, 0, len(validationErrors))
	for _, e := range validationErrors {
		messages = append(messages, formatValidationError(e))
	}

	return NewError(http.StatusBadRequest, strings.Join(messages, "; "))
}

// formatValidationError 将校验错误转为可读消息
func formatValidationError(e validator.FieldError) string {
	field := e.Field()
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s 不能为空", field)
	case "email":
		return fmt.Sprintf("%s 格式不正确", field)
	case "min":
		return fmt.Sprintf("%s 长度不能少于 %s", field, e.Param())
	case "max":
		return fmt.Sprintf("%s 长度不能超过 %s", field, e.Param())
	case "len":
		return fmt.Sprintf("%s 长度必须为 %s", field, e.Param())
	case "oneof":
		return fmt.Sprintf("%s 必须是 [%s] 之一", field, e.Param())
	default:
		return fmt.Sprintf("%s 校验失败 (%s)", field, e.Tag())
	}
}

// ValidatePassword 密码复杂度校验：含大写、小写、数字，长度从配置读取
func ValidatePassword(pwd string) error {
	minLen := 8
	if cfg := config.Get(); cfg != nil && cfg.Auth.MinPasswordLength > 0 {
		minLen = cfg.Auth.MinPasswordLength
	}
	if len(pwd) < minLen {
		return NewError(http.StatusBadRequest, fmt.Sprintf("密码长度至少%d位", minLen))
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range pwd {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return NewError(http.StatusBadRequest, "密码须包含大写字母、小写字母和数字")
	}
	return nil
}

// BindAndValidate 从请求解析并校验（适用于 JSON 请求体）
func BindAndValidate(r *http.Request, dst interface{}) error {
	if err := BindJSON(r, dst); err != nil {
		return NewError(http.StatusBadRequest, "请求体解析失败")
	}
	return ValidateStruct(dst)
}
