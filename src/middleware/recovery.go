package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"agent_flow/src/common"
	"agent_flow/src/config"
)

// Recovery Panic 恢复中间件，防止服务崩溃
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// debug 模式记录完整堆栈，release 模式只记录错误
				if config.Get().Server.Mode == "debug" {
					slog.Error("PANIC 恢复",
						"error", err,
						"path", r.URL.Path,
						"stack", string(debug.Stack()),
					)
				} else {
					slog.Error("PANIC 恢复", "error", err, "path", r.URL.Path)
				}
				common.JSONError(w, http.StatusInternalServerError, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
