package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap 支持 http.ResponseController 穿透 wrapper 访问底层连接
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// Logger 请求日志中间件（静态资源降为 DEBUG 级别）
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		ms := time.Since(start).Milliseconds()

		if strings.HasPrefix(r.URL.Path, "/static/") {
			slog.Debug("HTTP", "method", r.Method, "path", r.URL.Path, "status", sw.status, "ms", ms)
		} else {
			slog.Info("HTTP", "method", r.Method, "path", r.URL.Path, "status", sw.status, "size", sw.size, "ms", ms, "ip", r.RemoteAddr)
		}
	})
}
