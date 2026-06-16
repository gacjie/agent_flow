package controller

import (
	"net/http"
	"strconv"

	"agent_flow/src/common"
	"agent_flow/src/model"
	"agent_flow/src/tokenutil"

	"github.com/go-chi/chi/v5"
)

// CreateWorkspace 创建工作区（JSON）
func (c *WorkbenchCtrl) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		Description string `json:"description"`
		AgentID     uint   `json:"agent_id"`
		ProjectID   uint   `json:"project_id"`
	}

	if err := common.BindJSON(r, &req); err != nil {
		common.JSONError(w, http.StatusBadRequest, "参数错误: "+err.Error())
		return
	}

	if req.Name == "" {
		common.JSONError(w, http.StatusBadRequest, "名称不能为空")
		return
	}
	if req.Label == "" {
		req.Label = req.Name
	}

	createReq := &model.WorkspaceCreateReq{
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Label:       req.Label,
		Description: req.Description,
		AgentID:     req.AgentID,
	}

	workspace, err := c.WorkspaceService.Create(createReq)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "创建工作区失败: "+err.Error())
		return
	}

	common.JSONCreated(w, workspace)
}

// ListTasks 获取工作区任务列表（JSON）
func (c *WorkbenchCtrl) ListTasks(w http.ResponseWriter, r *http.Request) {
	wsIDStr := r.URL.Query().Get("workspace_id")
	if wsIDStr == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 不能为空")
		return
	}

	wsID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工作区 ID")
		return
	}

	ws, err := c.WorkspaceService.GetByID(uint(wsID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	tasks, err := c.TaskService.ListWithSubTasks(ws.UUID)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "获取任务列表失败")
		return
	}

	stats := c.TaskService.CountByWorkspace(ws.UUID)

	var taskSummaryTokens int
	if phase := c.TaskService.GetCurrentPhase(ws.UUID); phase > 0 {
		summary := c.TaskService.GetPhaseTasksSummary(ws.UUID, phase)
		taskSummaryTokens = tokenutil.EstimateText(summary)
	}

	common.JSONSuccess(w, map[string]interface{}{
		"tasks":               tasks,
		"stats":               stats,
		"task_summary_tokens": taskSummaryTokens,
	})
}

// ListWorkspaces APP 专用：列出所有进行中的工作区
func (c *WorkbenchCtrl) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	workspaces, err := c.WorkspaceService.ListActive()
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "获取工作区失败")
		return
	}
	common.JSONSuccess(w, workspaces)
}

// DeleteWorkspace 删除工作区（含所有会话和工作文件，不影响关联项目）
func (c *WorkbenchCtrl) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	wsID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工作区 ID")
		return
	}

	ws, err := c.WorkspaceService.GetByID(uint(wsID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	if c.RunnerManager != nil && c.RunnerManager.IsWorkspaceRunning(ws.UUID) {
		common.JSONError(w, http.StatusConflict, "工作区有正在运行的会话，请先停止后再删除")
		return
	}

	if err := c.WorkspaceService.Delete(ws.ID); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "删除工作区失败: "+err.Error())
		return
	}

	common.JSONSuccess(w, nil)
}
