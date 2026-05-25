package controller

import (
	"net/http"
	"strconv"

	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// TaskCtrl 任务展示控制器（只读，任务由 AI 工具负责创建和更新）
type TaskCtrl struct {
	Base
	TaskService      *service.TaskService
	WorkspaceService *service.WorkspaceService
}

// List 工作区任务展示页面（分阶段树形展示）
func (c *TaskCtrl) List(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := strconv.ParseUint(chi.URLParam(r, "workspaceID"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的工作区 ID")
		return
	}

	ws, err := c.WorkspaceService.GetByID(uint(workspaceID))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	// 从工作区 working.db 加载所有任务（含子任务，按阶段排序）
	tasks, err := c.TaskService.ListWithSubTasks(ws.UUID)
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取任务列表失败")
		return
	}

	stats := c.TaskService.CountByWorkspace(ws.UUID)

	c.Render(w, r, "admin/workspace_tasks", map[string]interface{}{
		"Title":      ws.Label + " - 任务列表",
		"ActiveMenu": "projects",
		"Workspace":  ws,
		"Tasks":      tasks,
		"Stats":      stats,
	})
}
