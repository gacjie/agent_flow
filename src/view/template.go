package view

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

// Engine 模板引擎
type Engine struct {
	templates map[string]*template.Template
	funcMap   template.FuncMap
}

// NewEngine 创建模板引擎并解析所有模板
func NewEngine() (*Engine, error) {
	e := &Engine{
		templates: make(map[string]*template.Template),
		funcMap: template.FuncMap{
			"formatTime": func(t time.Time) string {
				return t.Format("2006-01-02 15:04:05")
			},
			"formatDate": func(t time.Time) string {
				return t.Format("2006-01-02")
			},
			"truncate": func(s string, n int) string {
				runes := []rune(s)
				if len(runes) <= n {
					return s
				}
				return string(runes[:n]) + "..."
			},
			"add": func(a, b int) int {
				return a + b
			},
			"sub": func(a, b int) int {
				return a - b
			},
			"seq": func(start, end int) []int {
				s := make([]int, 0, end-start+1)
				for i := start; i <= end; i++ {
					s = append(s, i)
				}
				return s
			},
		},
	}

	if err := e.parseTemplates(); err != nil {
		return nil, err
	}
	return e, nil
}

// parseTemplates 解析所有模板文件（布局 + 页面）
func (e *Engine) parseTemplates() error {
	// 读取布局文件
	layoutFiles, err := fs.Glob(TemplateFS, "template/layout/*.html")
	if err != nil {
		return fmt.Errorf("读取布局模板失败: %w", err)
	}

	// 普通页面目录（使用 base 布局）
	normalDirs := []string{"error", "auth"}
	for _, dir := range normalDirs {
		if err := e.parseDir(dir, layoutFiles, "base"); err != nil {
			return err
		}
	}

	// 管理后台页面（使用 admin_base 布局）
	if err := e.parseDir("admin", layoutFiles, "admin_base"); err != nil {
		return err
	}

	return nil
}

// parseDir 解析指定目录下的模板
func (e *Engine) parseDir(dir string, layoutFiles []string, layoutName string) error {
	pattern := fmt.Sprintf("template/%s/*.html", dir)
	pages, err := fs.Glob(TemplateFS, pattern)
	if err != nil {
		return fmt.Errorf("读取页面模板失败 (%s): %w", dir, err)
	}

	for _, page := range pages {
		files := append(append([]string{}, layoutFiles...), page)
		tmpl, err := template.New("").Funcs(e.funcMap).ParseFS(TemplateFS, files...)
		if err != nil {
			return fmt.Errorf("解析模板失败 (%s): %w", page, err)
		}

		name := page[len("template/") : len(page)-len(".html")]
		e.templates[name] = tmpl
	}
	return nil
}

// Render 渲染模板到 ResponseWriter
func (e *Engine) Render(w http.ResponseWriter, name string, data interface{}) error {
	tmpl, ok := e.templates[name]
	if !ok {
		return fmt.Errorf("模板不存在: %s", name)
	}

	// admin/ 目录使用 admin_base 布局，其余使用 base
	layoutName := "base"
	if strings.HasPrefix(name, "admin/") {
		layoutName = "admin_base"
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, layoutName, data); err != nil {
		return fmt.Errorf("模板渲染失败 (%s): %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := buf.WriteTo(w)
	return err
}

// RenderError 渲染错误页面
func (e *Engine) RenderError(w http.ResponseWriter, code int, message string) {
	w.WriteHeader(code)
	data := map[string]interface{}{
		"Code":    code,
		"Message": message,
	}
	if err := e.Render(w, "error/error", data); err != nil {
		http.Error(w, message, code)
	}
}
