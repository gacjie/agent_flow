package common

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent_flow/src/config"
)

// Pagination 分页信息
type Pagination struct {
	Page      int `json:"page"`
	PageSize  int `json:"page_size"`
	Total     int `json:"total"`
	TotalPage int `json:"total_page"`
}

// NewPagination 创建分页（从请求参数读取 page 和 page_size）
func NewPagination(r *http.Request, total int) Pagination {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	defaultSize := 20
	maxSize := 100
	if cfg := config.Get(); cfg != nil {
		defaultSize = cfg.Pagination.DefaultPageSize
		maxSize = cfg.Pagination.MaxPageSize
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxSize {
		pageSize = defaultSize
	}

	totalPage := int(math.Ceil(float64(total) / float64(pageSize)))

	return Pagination{
		Page:      page,
		PageSize:  pageSize,
		Total:     total,
		TotalPage: totalPage,
	}
}

// Offset 计算数据库查询偏移量
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// FormatTime 格式化时间为本地字符串
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// TruncateString 截断字符串到指定长度，超出部分用 ... 替代
func TruncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// TrimStrings 去除字符串切片中每个元素的首尾空白
func TrimStrings(ss []string) []string {
	result := make([]string, len(ss))
	for i, s := range ss {
		result[i] = strings.TrimSpace(s)
	}
	return result
}

// BindJSON 从请求体解析 JSON
func BindJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

// BindForm 从表单解析参数到 map
func BindForm(r *http.Request) (map[string]string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for key, values := range r.PostForm {
		if len(values) > 0 {
			result[key] = strings.TrimSpace(values[0])
		}
	}
	return result, nil
}
