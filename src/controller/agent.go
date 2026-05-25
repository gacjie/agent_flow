package controller

import (
	"encoding/json"
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

// AgentCtrl AI 智能体管理控制器（/admin/agents）
type AgentCtrl struct {
	Base
	Service      *service.AgentService
	ModelService *service.LLMModelService
	SkillService *service.SkillService
	ToolService  *service.ToolService
}

// List AI 智能体列表页面
func (c *AgentCtrl) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := config.Get().Pagination.DefaultPageSize

	items, total, err := c.Service.List(page, pageSize)
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取 AI 智能体列表失败")
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
		"Title":      "AI 智能体管理",
		"Agents":     items,
		"Paging":     paging,
		"AdminPage":  true,
		"ActiveMenu": "agents",
	}
	c.Render(w, r, "admin/agent_list", data)
}

// CreateForm 创建 AI 智能体表单页面
func (c *AgentCtrl) CreateForm(w http.ResponseWriter, r *http.Request) {
	models, _ := c.ModelService.ListAll()
	skills, _ := c.SkillService.ListAll()

	// 默认勾选所有内置工具
	availableTools := c.getAvailableTools()
	defaultToolsMap := make(map[string]bool)
	for _, t := range availableTools {
		if t.IsBuiltin {
			defaultToolsMap[t.Name] = true
		}
	}

	data := map[string]interface{}{
		"Title":            "创建 AI 智能体",
		"Action":           "/admin/agents",
		"Method":           "POST",
		"FormItem":         model.Agent{Status: 1},
		"Models":           models,
		"Skills":           skills,
		"SkillIDMap":       map[uint]bool{},
		"AutoLoadFilesMap": map[string]bool{"role": true, "memory": true, "rule": true, "skill": true, "workflow": true},
		"AutoLoadToolsMap": defaultToolsMap,
		"AvailableTools":   availableTools,
		"Content":          map[string]string{},
		"AdminPage":        true,
		"ActiveMenu":       "agents",
	}
	c.Render(w, r, "admin/agent_form", data)
}

// Create 创建 AI 智能体
func (c *AgentCtrl) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	modelMode, modelID := parseModelSelection(r.FormValue("model_id"))

	// 解析技能 ID 列表
	var skillIDs []uint
	for _, idStr := range r.Form["skill_ids"] {
		if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
			skillIDs = append(skillIDs, uint(id))
		}
	}

	req := &model.AgentCreateReq{
		Name:            r.FormValue("name"),
		Title:           r.FormValue("title"),
		Description:     r.FormValue("description"),
		Keywords:        r.FormValue("keywords"),
		ModelID:         modelID,
		ModelMode:       modelMode,
		RolePrompt:      r.FormValue("role_prompt"),
		MemoryContent:   r.FormValue("memory_content"),
		RuleContent:     r.FormValue("rule_content"),
		SkillContent:    r.FormValue("skill_content"),
		WorkflowContent: r.FormValue("workflow_content"),
		AutoLoadFiles:   marshalCheckboxValues(r.Form["auto_load_files"]),
		AutoLoadTools:   marshalCheckboxValues(r.Form["auto_load_tools"]),
		Icon:            r.FormValue("icon"),
		SkillIDs:        skillIDs,
	}

	newAgent, err := c.Service.Create(req)
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "创建 AI 智能体失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:创建AI智能体", "operator", operator.Username, "agent", newAgent.Name)

	c.Redirect(w, r, "/admin/agents")
}

// EditForm 编辑 AI 智能体表单页面
func (c *AgentCtrl) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	agent, err := c.Service.GetByID(uint(id))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "AI 智能体不存在")
		return
	}

	// 从文件系统读取内容字段（供表单回显）
	content := c.Service.GetAgentContent(agent.Name)

	models, _ := c.ModelService.ListAll()
	skills, _ := c.SkillService.ListAll()
	skillIDs := c.Service.GetSkillIDs(uint(id))
	skillIDMap := make(map[uint]bool)
	for _, sid := range skillIDs {
		skillIDMap[sid] = true
	}

	data := map[string]interface{}{
		"Title":            "编辑 AI 智能体",
		"Action":           "/admin/agents/" + chi.URLParam(r, "id"),
		"Method":           "PUT",
		"FormItem":         agent,
		"Content":          content,
		"Models":           models,
		"Skills":           skills,
		"SkillIDMap":       skillIDMap,
		"AutoLoadFilesMap": autoLoadFilesMapOrDefault(agent.AutoLoadFiles),
		"AutoLoadToolsMap": parseJSONArrayToMap(agent.AutoLoadTools),
		"AvailableTools":   c.getAvailableTools(),
		"AdminPage":        true,
		"ActiveMenu":       "agents",
	}
	c.Render(w, r, "admin/agent_form", data)
}

// Update 更新 AI 角色
func (c *AgentCtrl) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	modelMode, modelIDVal := parseModelSelection(r.FormValue("model_id"))
	status, _ := strconv.Atoi(r.FormValue("status"))
	rolePrompt := r.FormValue("role_prompt")
	memoryContent := r.FormValue("memory_content")
	ruleContent := r.FormValue("rule_content")
	skillContent := r.FormValue("skill_content")
	workflowContent := r.FormValue("workflow_content")
	autoLoadFiles := marshalCheckboxValues(r.Form["auto_load_files"])
	autoLoadTools := marshalCheckboxValues(r.Form["auto_load_tools"])
	icon := r.FormValue("icon")

	// 解析技能 ID 列表
	skillIDs := make([]uint, 0)
	for _, idStr := range r.Form["skill_ids"] {
		if sid, err := strconv.ParseUint(idStr, 10, 64); err == nil {
			skillIDs = append(skillIDs, uint(sid))
		}
	}

	req := &model.AgentUpdateReq{
		Title:           r.FormValue("title"),
		Description:     r.FormValue("description"),
		Keywords:        r.FormValue("keywords"),
		ModelID:         &modelIDVal,
		ModelMode:       &modelMode,
		RolePrompt:      &rolePrompt,
		MemoryContent:   &memoryContent,
		RuleContent:     &ruleContent,
		SkillContent:    &skillContent,
		WorkflowContent: &workflowContent,
		AutoLoadFiles:   &autoLoadFiles,
		AutoLoadTools:   &autoLoadTools,
		Icon:            &icon,
		Status:          &status,
		SkillIDs:        skillIDs,
	}

	if _, err := c.Service.Update(uint(id), req); err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "更新 AI 智能体失败")
		return
	}

	c.Redirect(w, r, "/admin/agents")
}

// Delete 删除 AI 角色
func (c *AgentCtrl) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := c.Service.Delete(uint(id)); err != nil {
		c.RenderError(w, http.StatusInternalServerError, "删除 AI 角色失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:删除AI智能体", "operator", operator.Username, "agent_id", id)

	common.JSONSuccess(w, nil)
}

// getAvailableTools 获取启用的工具列表（供表单模板渲染）
func (c *AgentCtrl) getAvailableTools() []service.ToolInfo {
	if c.ToolService == nil {
		return nil
	}
	var result []service.ToolInfo
	for _, t := range c.ToolService.ListAvailable() {
		if t.Status == 1 {
			result = append(result, t)
		}
	}
	return result
}

// autoLoadFilesMapOrDefault 解析 AutoLoadFiles JSON 为 map；若为空则返回全选默认值
func autoLoadFilesMapOrDefault(jsonStr string) map[string]bool {
	if jsonStr == "" {
		return map[string]bool{"role": true, "memory": true, "rule": true, "skill": true, "workflow": true}
	}
	return parseJSONArrayToMap(jsonStr)
}

// parseJSONArrayToMap 将 JSON 数组字符串解析为 map（用于模板 checked 判断）
func parseJSONArrayToMap(jsonStr string) map[string]bool {
	result := make(map[string]bool)
	if jsonStr == "" {
		return result
	}
	var items []string
	if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
		return result
	}
	for _, item := range items {
		result[item] = true
	}
	return result
}

// marshalCheckboxValues 将复选框多选值序列化为 JSON 数组字符串
func marshalCheckboxValues(values []string) string {
	if len(values) == 0 {
		return ""
	}
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(data)
}

// parseModelSelection 解析模型选择表单值
// 表单 value 为 "auto"/"default"/model_id字符串
// 返回 (modelMode, modelID)
func parseModelSelection(val string) (string, string) {
	switch val {
	case "auto":
		return "auto", "auto"
	case "default":
		return "default", "default"
	default:
		return "", val
	}
}
