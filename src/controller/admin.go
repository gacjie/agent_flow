package controller

import (
	"net/http"

	"agent_flow/src/service"
)

// AdminDashboard 管理面板控制器
type AdminDashboard struct {
	Base
	AdminService     *service.AdminService
	PermService      *service.PermissionService
	ProjectService   *service.ProjectService
	AgentService     *service.AgentService
	SkillService     *service.SkillService
	WorkspaceService *service.WorkspaceService
	ModelService     *service.LLMModelService
	ToolService      *service.ToolService
}

// Dashboard 管理面板首页
func (c *AdminDashboard) Dashboard(w http.ResponseWriter, r *http.Request) {
	roles, _ := c.PermService.ListRoles()
	perms, _ := c.PermService.ListPermissions()

	// 工具统计
	systemTools, _ := c.ToolService.ListSystemTools()
	mcpTools, _ := c.ToolService.ListMCPTools()
	var systemToolEnabled, mcpToolEnabled int
	for _, t := range systemTools {
		if t.Status == 1 {
			systemToolEnabled++
		}
	}
	for _, t := range mcpTools {
		if t.Status == 1 {
			mcpToolEnabled++
		}
	}

	// 活跃工作区（取前 5）
	activeWorkspaces, _ := c.WorkspaceService.ListActive()
	if len(activeWorkspaces) > 5 {
		activeWorkspaces = activeWorkspaces[:5]
	}

	// 最近项目（前 5）
	recentProjects, _, _ := c.ProjectService.List(1, 5)

	// auto 模型数量
	autoModels, _ := c.ModelService.GetAutoModels()

	data := map[string]interface{}{
		"Title":      "管理面板",
		"AdminPage":  true,
		"ActiveMenu": "dashboard",
		// 主要统计
		"ProjectCount":   c.ProjectService.Count(),
		"AgentCount":     c.AgentService.Count(),
		"SkillCount":     c.SkillService.Count(),
		"WorkspaceCount": c.WorkspaceService.Count(),
		// 次要统计
		"ModelCount":        c.ModelService.Count(),
		"AdminCount":        c.AdminService.Count(),
		"RoleCount":         len(roles),
		"PermCount":         len(perms),
		"SystemToolCount":   len(systemTools),
		"SystemToolEnabled": systemToolEnabled,
		"MCPToolCount":      len(mcpTools),
		"MCPToolEnabled":    mcpToolEnabled,
		// 列表数据
		"ActiveWorkspaces": activeWorkspaces,
		"RecentProjects":   recentProjects,
		// 系统信息
		"AutoModelCount": len(autoModels),
	}
	c.Render(w, r, "admin/dashboard", data)
}
