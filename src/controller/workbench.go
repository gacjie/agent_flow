package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"agent_flow/src/model"
	"agent_flow/src/service"
)

// WorkbenchCtrl 工作台控制器
type WorkbenchCtrl struct {
	Base
	ChatService      *service.ChatService
	ChatRunner       *service.ChatRunner
	WorkspaceService *service.WorkspaceService
	AgentService     *service.AgentService
	ProjectService   *service.ProjectService
	ModelService     *service.LLMModelService
	RunnerManager    *service.RunnerManager
	TaskService      *service.TaskService
}

// Page 工作台主页面
func (c *WorkbenchCtrl) Page(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := strconv.ParseUint(r.URL.Query().Get("workspace_id"), 10, 64)
	convIDStr := r.URL.Query().Get("conv_id")

	workspaces, _ := c.WorkspaceService.ListActive()
	agents, _ := c.AgentService.ListAll()
	projects, _ := c.ProjectService.ListAll()
	models, _ := c.ModelService.ListAll()

	defaultModelID := ""
	for _, m := range models {
		if m.IsDefault {
			defaultModelID = m.ModelID
			break
		}
	}

	data := map[string]interface{}{
		"Title":          "工作台",
		"ActiveMenu":     "workbench",
		"Workspaces":     workspaces,
		"Agents":         agents,
		"Projects":       projects,
		"Models":         models,
		"DefaultModelID": defaultModelID,
	}

	if workspaceID > 0 {
		ws, err := c.WorkspaceService.GetByID(uint(workspaceID))
		if err == nil {
			data["Workspace"] = ws
			sessionSvc := c.ChatService.ForWorkspace(ws.UUID)

			// 优先使用 URL 中的 conv_id
			if convIDStr != "" {
				convID, err := strconv.ParseUint(convIDStr, 10, 64)
				if err == nil {
					// 验证会话属于当前工作区
					var conv model.Conversation
					sk := service.SessionKey(ws.UUID, uint(convID))
					if err := sessionSvc.DB.Where("id = ? AND workspace_id = ?",
						uint(convID), uint(workspaceID)).First(&conv).Error; err == nil {
						data["ActiveConvID"] = conv.ID
						data["ConvAgentID"] = conv.AgentID
						// 检查是否有运行中的任务
						if c.RunnerManager != nil && c.RunnerManager.IsRunning(sk) {
							data["IsRunning"] = true
							data["LastSeq"] = c.RunnerManager.GetCurrentSeq(sk)
						}
					}
				}
			} else {
				// 无 conv_id：查找活跃主会话并重定向
				var activeConv model.Conversation
				if err := sessionSvc.DB.Where("workspace_id = ? AND status = 1 AND conv_type = ?",
					uint(workspaceID), "main").First(&activeConv).Error; err == nil {
					http.Redirect(w, r, fmt.Sprintf("/admin/workbench?workspace_id=%d&conv_id=%d",
						workspaceID, activeConv.ID), http.StatusFound)
					return
				}
			}
		}
	}

	c.Render(w, r, "admin/workbench", data)
}
