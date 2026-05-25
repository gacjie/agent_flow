package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
)

const nonceCtxKey = ctxKey("csp_nonce")

// Security 安全响应头中间件
func Security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 生成 CSP nonce
		nonce := generateNonce()
		ctx := context.WithValue(r.Context(), nonceCtxKey, nonce)
		r = r.WithContext(ctx)

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", fmt.Sprintf(
			"default-src 'self'; script-src 'self' 'nonce-%s'; style-src 'self' 'unsafe-inline'; img-src 'self' data:",
			nonce,
		))

		// HSTS：仅 HTTPS 请求时设置
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// GetCSPNonce 获取当前请求的 CSP nonce
func GetCSPNonce(r *http.Request) string {
	if nonce, ok := r.Context().Value(nonceCtxKey).(string); ok {
		return nonce
	}
	return ""
}

// generateNonce 生成 16 字节的 base64 编码 nonce
func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}
