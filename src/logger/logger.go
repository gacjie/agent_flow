package logger

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MultiHandler 将日志同时分发给多个 slog.Handler
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler 创建多路分发 Handler
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	return &MultiHandler{handlers: handlers}
}

func (h *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &MultiHandler{handlers: handlers}
}

func (h *MultiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &MultiHandler{handlers: handlers}
}

// DailyFileHandler 按日期轮转的文件日志 Handler，只写入指定级别及以上的日志
type DailyFileHandler struct {
	mu       sync.Mutex
	dir      string
	minLevel slog.Level
	date     string       // 当前已打开文件对应的日期（2006-01-02）
	file     *os.File
	inner    slog.Handler // 实际写入文件的 slog.Handler（TextHandler）
}

// NewDailyFileHandler 创建按天轮转的文件 Handler
// dir: 日志目录（如 logs/error）
// minLevel: 最低写入级别（如 slog.LevelWarn）
func NewDailyFileHandler(dir string, minLevel slog.Level) *DailyFileHandler {
	return &DailyFileHandler{
		dir:      dir,
		minLevel: minLevel,
	}
}

// rotate 检查日期是否变化，必要时切换到新文件（调用方需持锁）
func (h *DailyFileHandler) rotate() error {
	today := time.Now().Format("2006-01-02")
	if h.date == today && h.file != nil {
		return nil
	}
	// 关闭旧文件
	if h.file != nil {
		_ = h.file.Close()
		h.file = nil
		h.inner = nil
	}
	// 创建新文件
	path := filepath.Join(h.dir, today+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	h.file = f
	h.date = today
	h.inner = slog.NewTextHandler(f, &slog.HandlerOptions{
		Level:     h.minLevel,
		AddSource: false,
	})
	return nil
}

func (h *DailyFileHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *DailyFileHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.rotate(); err != nil {
		// 轮转失败时静默忽略，不影响主流程
		return nil
	}
	return h.inner.Handle(ctx, r)
}

func (h *DailyFileHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// 返回新实例，共享同一个文件句柄（通过 mu 保护）
	n := &DailyFileHandler{
		dir:      h.dir,
		minLevel: h.minLevel,
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	n.date = h.date
	n.file = h.file
	if h.inner != nil {
		n.inner = h.inner.WithAttrs(attrs)
	}
	return n
}

func (h *DailyFileHandler) WithGroup(name string) slog.Handler {
	n := &DailyFileHandler{
		dir:      h.dir,
		minLevel: h.minLevel,
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	n.date = h.date
	n.file = h.file
	if h.inner != nil {
		n.inner = h.inner.WithGroup(name)
	}
	return n
}

// Close 关闭当前打开的日志文件，应在程序退出时调用
func (h *DailyFileHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.file != nil {
		err := h.file.Close()
		h.file = nil
		h.inner = nil
		return err
	}
	return nil
}
