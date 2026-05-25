package controller

import (
	"log/slog"
	"net/http"
	"strconv"

	"agent_flow/src/middleware"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// SystemToolCtrl 系统工具管理控制器
type SystemToolCtrl struct {
	Base
	ToolService *service.ToolService
}

// List 系统工具列表页面
func (c *SystemToolCtrl) List(w http.ResponseWriter, r *http.Request) {
	tools, err := c.ToolService.ListSystemTools()
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取系统工具列表失败")
		return
	}

	data := map[string]interface{}{
		"Title":      "系统工具",
		"Tools":      tools,
		"AdminPage":  true,
		"ActiveMenu": "system-tools",
	}
	c.Render(w, r, "admin/system_tool_list", data)
}

// Toggle 切换系统工具启用/禁用状态
func (c *SystemToolCtrl) Toggle(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的工具 ID")
		return
	}

	if err := c.ToolService.ToggleSystemToolStatus(uint(id)); err != nil {
		c.RenderError(w, http.StatusInternalServerError, "切换工具状态失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:切换系统工具状态", "operator", operator.Username, "tool_id", id)

	c.Redirect(w, r, "/admin/system-tools")
}
