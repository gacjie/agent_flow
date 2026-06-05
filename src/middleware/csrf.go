package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"agent_flow/src/common"
)

const (
	csrfCtxKey = ctxKey("csrf_token")
)

// CSRF 防护中间件：GET 请求种 Token，写请求校验 Token
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 读取或生成 CSRF Token
		token := getCSRFToken(r)
		if token == "" {
			token = generateCSRFToken()
			http.SetCookie(w, &http.Cookie{
				Name:     common.CSRFCookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: false,
				SameSite: http.SameSiteStrictMode,
			})
		}

		// 将 token 存入 context，供模板渲染时读取
		ctx := context.WithValue(r.Context(), csrfCtxKey, token)
		r = r.WithContext(ctx)

		// 安全方法（GET/HEAD/OPTIONS）跳过校验
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Bearer Token 请求跳过 CSRF（Token 本身防 CSRF）
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		// 写操作校验 CSRF Token
		formToken := r.FormValue(common.CSRFFieldName)
		if formToken == "" {
			formToken = r.Header.Get("X-CSRF-Token")
		}

		if formToken != token {
			if isAPIRequest(r) {
				common.JSONError(w, http.StatusForbidden, "CSRF Token 校验失败")
			} else {
				common.JSONError(w, http.StatusForbidden, "请求已过期，请刷新页面重试")
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetCSRFTokenFromRequest 获取当前请求的 CSRF Token（优先 context，其次 Cookie）
func GetCSRFTokenFromRequest(r *http.Request) string {
	if token, ok := r.Context().Value(csrfCtxKey).(string); ok && token != "" {
		return token
	}
	return getCSRFToken(r)
}

// getCSRFToken 从 Cookie 中读取 CSRF Token
func getCSRFToken(r *http.Request) string {
	cookie, err := r.Cookie(common.CSRFCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// generateCSRFToken 生成随机 CSRF Token
func generateCSRFToken() string {
	bytes := make([]byte, common.CSRFTokenLen)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
