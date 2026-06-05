package middleware

import (
	"context"
	"net/http"
	"strings"

	"agent_flow/src/common"
	"agent_flow/src/model"
	"agent_flow/src/service"
)

type ctxKey string

const (
	AdminKey       ctxKey = "current_admin"
	PermissionsKey ctxKey = "permissions"
)

// Auth 认证中间件：必须登录 + 强制改密检查
func Auth(authService *service.AuthService, permService *service.PermissionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admin := getAdminFromCookie(r, authService)
			if admin == nil {
				if isAPIRequest(r) {
					common.JSONError(w, http.StatusUnauthorized, "请先登录")
				} else {
					http.Redirect(w, r, "/auth/login", http.StatusFound)
				}
				return
			}

			perms := permService.GetPermissionMap(admin.RoleID)
			ctx := context.WithValue(r.Context(), AdminKey, admin)
			ctx = context.WithValue(ctx, PermissionsKey, perms)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission 权限检查中间件
func RequirePermission(code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			perms := GetPermissions(r)
			if !perms[code] {
				if isAPIRequest(r) {
					common.JSONError(w, http.StatusForbidden, "无权限访问")
				} else {
					common.JSONError(w, http.StatusForbidden, "无权限访问此功能")
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// InjectAdmin 非强制认证：尝试注入管理员和权限信息到 context
func InjectAdmin(authService *service.AuthService, permService *service.PermissionService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admin := getAdminFromCookie(r, authService)
			if admin != nil {
				perms := permService.GetPermissionMap(admin.RoleID)
				ctx := context.WithValue(r.Context(), AdminKey, admin)
				ctx = context.WithValue(ctx, PermissionsKey, perms)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetCurrentAdmin 从 context 中获取当前登录管理员
func GetCurrentAdmin(r *http.Request) *model.Admin {
	admin, _ := r.Context().Value(AdminKey).(*model.Admin)
	return admin
}

// GetPermissions 从 context 中获取当前权限
func GetPermissions(r *http.Request) map[string]bool {
	perms, _ := r.Context().Value(PermissionsKey).(map[string]bool)
	if perms == nil {
		return make(map[string]bool)
	}
	return perms
}

// getAdminFromCookie 从 Cookie 读取 session token 并验证（含 IP 校验）
func getAdminFromCookie(r *http.Request, authService *service.AuthService) *model.Admin {
	// 1. Bearer Token（APP，无 IP 校验）
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		if admin, err := authService.ValidateAPIToken(auth[7:]); err == nil {
			return admin
		}
	}
	// 2. Cookie Session（浏览器，含 IP 校验）
	cookie, err := r.Cookie(common.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}
	admin, err := authService.ValidateSession(cookie.Value, r.RemoteAddr)
	if err != nil {
		return nil
	}
	return admin
}

// isAPIRequest 判断是否为 API 请求
func isAPIRequest(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json")
}
