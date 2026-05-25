package controller

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/middleware"
	"agent_flow/src/model"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// ProjectCtrl 项目管理控制器（/admin/projects）
type ProjectCtrl struct {
	Base
	Service *service.ProjectService
}

// List 项目列表页面
func (c *ProjectCtrl) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := config.Get().Pagination.DefaultPageSize

	items, total, err := c.Service.List(page, pageSize)
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取项目列表失败")
		return
	}

	paging := common.Pagination{
		Page:     page,
		PageSize: pageSize,
		Total:    int(total),
	}
	if paging.PageSize > 0 {
		paging.TotalPage = (paging.Total + paging.PageSize - 1) / paging.PageSize
	}

	data := map[string]interface{}{
		"Title":      "项目管理",
		"Projects":   items,
		"Paging":     paging,
		"AdminPage":  true,
		"ActiveMenu": "projects",
	}
	c.Render(w, r, "admin/project_list", data)
}

// CreateForm 创建项目表单页面
func (c *ProjectCtrl) CreateForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":      "创建项目",
		"Action":     "/admin/projects",
		"Method":     "POST",
		"FormItem":   model.Project{Status: 1},
		"AdminPage":  true,
		"ActiveMenu": "projects",
	}
	c.Render(w, r, "admin/project_form", data)
}

// Create 创建项目
func (c *ProjectCtrl) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	req := &model.ProjectCreateReq{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Path:        r.FormValue("path"),
	}

	newProject, err := c.Service.Create(req)
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		slog.Error("创建项目失败", "error", err)
		c.RenderError(w, http.StatusInternalServerError, "创建项目失败: "+err.Error())
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:创建项目", "operator", operator.Username, "project", newProject.Name)

	c.Redirect(w, r, "/admin/projects")
}

// EditForm 编辑项目表单页面
func (c *ProjectCtrl) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	project, err := c.Service.GetByID(uint(id))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "项目不存在")
		return
	}

	data := map[string]interface{}{
		"Title":      "编辑项目",
		"Action":     "/admin/projects/" + chi.URLParam(r, "id"),
		"Method":     "PUT",
		"FormItem":   project,
		"AdminPage":  true,
		"ActiveMenu": "projects",
	}
	c.Render(w, r, "admin/project_form", data)
}

// Update 更新项目
func (c *ProjectCtrl) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	status, _ := strconv.Atoi(r.FormValue("status"))

	req := &model.ProjectUpdateReq{
		Description: r.FormValue("description"),
		Path:        r.FormValue("path"),
		Status:      &status,
	}

	if _, err := c.Service.Update(uint(id), req); err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		slog.Error("更新项目失败", "error", err)
		c.RenderError(w, http.StatusInternalServerError, "更新项目失败: "+err.Error())
		return
	}

	c.Redirect(w, r, "/admin/projects")
}

// Delete 删除项目
func (c *ProjectCtrl) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := c.Service.Delete(uint(id)); err != nil {
		c.RenderError(w, http.StatusInternalServerError, "删除项目失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:删除项目", "operator", operator.Username, "project_id", id)

	common.JSONSuccess(w, nil)
}

// BrowseDir 浏览服务器目录（供前端目录选择器使用）
func (c *ProjectCtrl) BrowseDir(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")

	type DirEntry struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}

	type BrowseResult struct {
		Current string     `json:"current"`
		Parent  string     `json:"parent"`
		Dirs    []DirEntry `json:"dirs"`
	}

	// 空路径：Windows 返回盘符列表，Linux 返回 /
	if dir == "" {
		if runtime.GOOS == "windows" {
			var drives []DirEntry
			for _, d := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
				drive := string(d) + ":\\"
				if _, err := os.Stat(drive); err == nil {
					drives = append(drives, DirEntry{Name: string(d) + ":", Path: drive})
				}
			}
			common.JSONSuccess(w, BrowseResult{Current: "", Parent: "", Dirs: drives})
			return
		}
		dir = "/"
	}

	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的路径")
		return
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无法读取目录: "+err.Error())
		return
	}

	var dirs []DirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" {
			continue
		}
		dirs = append(dirs, DirEntry{
			Name: name,
			Path: filepath.Join(absDir, name),
		})
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	parent := filepath.Dir(absDir)
	if parent == absDir {
		if runtime.GOOS == "windows" {
			parent = ""
		} else {
			parent = ""
		}
	}

	common.JSONSuccess(w, BrowseResult{Current: absDir, Parent: parent, Dirs: dirs})
}
