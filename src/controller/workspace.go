package controller

import (
	"log/slog"
	"net/http"
	"strconv"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/middleware"
	"agent_flow/src/model"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// WorkspaceCtrl 工作区管理控制器（/admin/projects/{projectID}/workspaces）
type WorkspaceCtrl struct {
	Base
	Service        *service.WorkspaceService
	ProjectService *service.ProjectService
	AgentService   *service.AgentService
}

// List 工作区列表页面
func (c *WorkspaceCtrl) List(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的项目 ID")
		return
	}

	project, err := c.ProjectService.GetByID(uint(projectID))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "项目不存在")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := config.Get().Pagination.DefaultPageSize

	items, total, err := c.Service.ListByProject(uint(projectID), page, pageSize)
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取工作区列表失败")
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
		"Title":      project.Name + " - 工作区",
		"Project":    project,
		"Workspaces": items,
		"Paging":     paging,
		"AdminPage":  true,
		"ActiveMenu": "projects",
	}
	c.Render(w, r, "admin/workspace_list", data)
}

// CreateForm 创建工作区表单页面
func (c *WorkspaceCtrl) CreateForm(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的项目 ID")
		return
	}

	project, err := c.ProjectService.GetByID(uint(projectID))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "项目不存在")
		return
	}

	agents, _ := c.AgentService.ListAll()

	data := map[string]interface{}{
		"Title":      "创建工作区",
		"Action":     "/admin/projects/" + chi.URLParam(r, "projectID") + "/workspaces",
		"Method":     "POST",
		"FormItem":   model.Workspace{ProjectID: uint(projectID), Status: 1},
		"Project":    project,
		"Agents":     agents,
		"AdminPage":  true,
		"ActiveMenu": "projects",
	}
	c.Render(w, r, "admin/workspace_form", data)
}

// Create 创建工作区
func (c *WorkspaceCtrl) Create(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的项目 ID")
		return
	}

	if _, err := c.ProjectService.GetByID(uint(projectID)); err != nil {
		c.RenderError(w, http.StatusNotFound, "项目不存在")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	agentID, _ := strconv.ParseUint(r.FormValue("agent_id"), 10, 64)

	req := &model.WorkspaceCreateReq{
		ProjectID:   uint(projectID),
		Name:        r.FormValue("name"),
		Label:       r.FormValue("label"),
		Description: r.FormValue("description"),
		AgentID:     uint(agentID),
	}

	newWs, err := c.Service.Create(req)
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "创建工作区失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:创建工作区", "operator", operator.Username, "workspace", newWs.Name)

	c.Redirect(w, r, "/admin/projects/"+chi.URLParam(r, "projectID")+"/workspaces")
}

// EditForm 编辑工作区表单页面
func (c *WorkspaceCtrl) EditForm(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseUint(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的项目 ID")
		return
	}

	project, err := c.ProjectService.GetByID(uint(projectID))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "项目不存在")
		return
	}

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	workspace, err := c.Service.GetByID(uint(id))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	agents, _ := c.AgentService.ListAll()

	data := map[string]interface{}{
		"Title":      "编辑工作区",
		"Action":     "/admin/projects/" + chi.URLParam(r, "projectID") + "/workspaces/" + chi.URLParam(r, "id"),
		"Method":     "PUT",
		"FormItem":   workspace,
		"Project":    project,
		"Agents":     agents,
		"AdminPage":  true,
		"ActiveMenu": "projects",
	}
	c.Render(w, r, "admin/workspace_form", data)
}

// Update 更新工作区
func (c *WorkspaceCtrl) Update(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")

	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	agentID, _ := strconv.ParseUint(r.FormValue("agent_id"), 10, 64)
	agentIDUint := uint(agentID)
	status, _ := strconv.Atoi(r.FormValue("status"))

	req := &model.WorkspaceUpdateReq{
		Label:       r.FormValue("label"),
		Description: r.FormValue("description"),
		AgentID:     &agentIDUint,
		Status:      &status,
	}

	if _, err := c.Service.Update(uint(id), req); err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "更新工作区失败")
		return
	}

	c.Redirect(w, r, "/admin/projects/"+projectID+"/workspaces")
}

// Delete 删除工作区
func (c *WorkspaceCtrl) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := c.Service.Delete(uint(id)); err != nil {
		c.RenderError(w, http.StatusInternalServerError, "删除工作区失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:删除工作区", "operator", operator.Username, "workspace_id", id)

	common.JSONSuccess(w, nil)
}
