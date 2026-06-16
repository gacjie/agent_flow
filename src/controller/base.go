package controller

import (
	"net/http"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/middleware"
	"agent_flow/src/view"
)

// Base 基础控制器，提供公共方法供其他控制器复用
type Base struct {
	View *view.Engine
}

// Render 渲染 HTML 模板（自动注入 Admin 和 CSRFToken）
func (b *Base) Render(w http.ResponseWriter, r *http.Request, name string, data map[string]interface{}) {
	if data == nil {
		data = make(map[string]interface{})
	}
	// 自动注入当前管理员（导航栏显示）
	if _, ok := data["Admin"]; !ok {
		data["Admin"] = middleware.GetCurrentAdmin(r)
	}
	// 自动注入权限（模板条件渲染）
	if _, ok := data["Permissions"]; !ok {
		data["Permissions"] = middleware.GetPermissions(r)
	}
	// 自动注入 CSRF Token（表单使用）
	if _, ok := data["CSRFToken"]; !ok {
		data["CSRFToken"] = middleware.GetCSRFTokenFromRequest(r)
	}
	// 自动注入 CSP Nonce（内联脚本使用）
	if _, ok := data["Nonce"]; !ok {
		data["Nonce"] = middleware.GetCSPNonce(r)
	}
	// 自动注入版本号（侧边栏展示）
	if _, ok := data["Version"]; !ok {
		data["Version"] = config.Version
	}
	if err := b.View.Render(w, name, data); err != nil {
		b.RenderError(w, http.StatusInternalServerError, "页面渲染失败")
	}
}

// RenderError 渲染错误页面
func (b *Base) RenderError(w http.ResponseWriter, code int, message string) {
	b.View.RenderError(w, code, message)
}

// JSON 输出 JSON 成功响应
func (b *Base) JSON(w http.ResponseWriter, data interface{}) {
	common.JSONSuccess(w, data)
}

// JSONError 输出 JSON 错误响应
func (b *Base) JSONError(w http.ResponseWriter, code int, message string) {
	common.JSONError(w, code, message)
}

// Redirect 重定向
func (b *Base) Redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusFound)
}
