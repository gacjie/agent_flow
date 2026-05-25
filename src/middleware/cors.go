package middleware

import (
	"net/http"
	"strings"

	"agent_flow/src/config"
)

// CORS 跨域中间件（从配置读取允许源）
func CORS() func(http.Handler) http.Handler {
	cfg := config.Get()
	origins := cfg.CORS.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:8080"}
	}

	methods := cfg.CORS.AllowedMethods
	if methods == "" {
		methods = "GET, POST, PUT, DELETE, OPTIONS"
	}
	headers := cfg.CORS.AllowedHeaders
	if headers == "" {
		headers = "Content-Type, Authorization, X-Requested-With, X-CSRF-Token"
	}
	allowOrigins := strings.Join(origins, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigins)
			w.Header().Set("Access-Control-Allow-Methods", methods)
			w.Header().Set("Access-Control-Allow-Headers", headers)
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
