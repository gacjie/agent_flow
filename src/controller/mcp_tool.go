package controller

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"agent_flow/src/common"
	"agent_flow/src/middleware"
	"agent_flow/src/model"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// MCPToolCtrl MCP 工具管理控制器
type MCPToolCtrl struct {
	Base
	ToolService *service.ToolService
}

// List MCP 工具列表页面
func (c *MCPToolCtrl) List(w http.ResponseWriter, r *http.Request) {
	tools, err := c.ToolService.ListMCPTools()
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取MCP工具列表失败")
		return
	}

	data := map[string]interface{}{
		"Title":      "MCP管理",
		"Tools":      tools,
		"AdminPage":  true,
		"ActiveMenu": "mcp-tools",
	}
	c.Render(w, r, "admin/mcp_tool_list", data)
}

// CreateForm 创建 MCP 工具表单
func (c *MCPToolCtrl) CreateForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":      "新建MCP工具",
		"Tool":       &model.MCPTool{Timeout: 30},
		"Action":     "/admin/mcp-tools",
		"AdminPage":  true,
		"ActiveMenu": "mcp-tools",
	}
	c.Render(w, r, "admin/mcp_tool_form", data)
}

// Create 创建 MCP 工具
func (c *MCPToolCtrl) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "解析表单失败")
		return
	}

	timeout, _ := strconv.Atoi(r.FormValue("timeout"))
	req := &model.MCPToolCreateReq{
		Name:        r.FormValue("name"),
		Label:       r.FormValue("label"),
		Description: r.FormValue("description"),
		Command:     r.FormValue("command"),
		Args:        r.FormValue("args"),
		Env:         r.FormValue("env"),
		Parameters:  r.FormValue("parameters"),
		Category:    r.FormValue("category"),
		Version:     r.FormValue("version"),
		Timeout:     timeout,
	}

	if err := common.ValidateStruct(req); err != nil {
		c.renderMCPForm(w, r, "新建MCP工具", "/admin/mcp-tools", err.Error(), &model.MCPTool{
			Name: req.Name, Label: req.Label, Description: req.Description,
			Command: req.Command, Args: req.Args, Env: req.Env, Parameters: req.Parameters,
			Category: req.Category, Version: req.Version, Timeout: req.Timeout,
		})
		return
	}

	if err := c.ToolService.CreateMCPTool(req); err != nil {
		c.renderMCPForm(w, r, "新建MCP工具", "/admin/mcp-tools", "创建失败："+err.Error(), &model.MCPTool{
			Name: req.Name, Label: req.Label, Description: req.Description,
			Command: req.Command, Args: req.Args, Env: req.Env, Parameters: req.Parameters,
			Category: req.Category, Version: req.Version, Timeout: req.Timeout,
		})
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:创建MCP工具", "operator", operator.Username, "name", req.Name)

	c.Redirect(w, r, "/admin/mcp-tools")
}

// EditForm 编辑 MCP 工具表单
func (c *MCPToolCtrl) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseMCPID(r)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的工具 ID")
		return
	}

	t, err := c.ToolService.GetMCPTool(id)
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "工具不存在")
		return
	}

	data := map[string]interface{}{
		"Title":      "编辑MCP工具",
		"Tool":       t,
		"ServerJSON": c.buildServerJSON(t),
		"Action":     "/admin/mcp-tools/" + strconv.FormatUint(uint64(id), 10),
		"AdminPage":  true,
		"ActiveMenu": "mcp-tools",
	}
	c.Render(w, r, "admin/mcp_tool_form", data)
}

// Update 更新 MCP 工具
func (c *MCPToolCtrl) Update(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseMCPID(r)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的工具 ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "解析表单失败")
		return
	}

	action := "/admin/mcp-tools/" + strconv.FormatUint(uint64(id), 10)
	serverJSONStr := r.FormValue("server_json")

	var entry model.MCPServerEntry
	if err := json.Unmarshal([]byte(serverJSONStr), &entry); err != nil {
		t, _ := c.ToolService.GetMCPTool(id)
		c.renderMCPFormWithJSON(w, r, "编辑MCP工具", action,
			"服务器配置 JSON 格式错误: "+err.Error(), t, serverJSONStr)
		return
	}
	if entry.Command == "" {
		t, _ := c.ToolService.GetMCPTool(id)
		c.renderMCPFormWithJSON(w, r, "编辑MCP工具", action,
			"command 不能为空", t, serverJSONStr)
		return
	}

	argsJSON := ""
	if len(entry.Args) > 0 {
		if b, err := json.Marshal(entry.Args); err == nil {
			argsJSON = string(b)
		}
	}
	envJSON := ""
	if len(entry.Env) > 0 {
		if b, err := json.Marshal(entry.Env); err == nil {
			envJSON = string(b)
		}
	}

	timeout, _ := strconv.Atoi(r.FormValue("timeout"))
	req := &model.MCPToolUpdateReq{
		Label:       r.FormValue("label"),
		Description: r.FormValue("description"),
		Command:     entry.Command,
		Args:        argsJSON,
		Env:         envJSON,
		Parameters:  r.FormValue("parameters"),
		Category:    r.FormValue("category"),
		Version:     r.FormValue("version"),
		Timeout:     timeout,
	}

	if err := common.ValidateStruct(req); err != nil {
		t, _ := c.ToolService.GetMCPTool(id)
		c.renderMCPFormWithJSON(w, r, "编辑MCP工具", action, err.Error(), t, serverJSONStr)
		return
	}

	if err := c.ToolService.UpdateMCPTool(id, req); err != nil {
		t, _ := c.ToolService.GetMCPTool(id)
		c.renderMCPFormWithJSON(w, r, "编辑MCP工具", action, "更新失败："+err.Error(), t, serverJSONStr)
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:更新MCP工具", "operator", operator.Username, "id", id)

	c.Redirect(w, r, "/admin/mcp-tools")
}

// Delete 删除 MCP 工具
func (c *MCPToolCtrl) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseMCPID(r)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工具 ID")
		return
	}

	if err := c.ToolService.DeleteMCPTool(id); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "删除失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:删除MCP工具", "operator", operator.Username, "id", id)

	common.JSONSuccess(w, nil)
}

// Toggle 切换 MCP 工具启用/禁用状态
func (c *MCPToolCtrl) Toggle(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseMCPID(r)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的工具 ID")
		return
	}

	if err := c.ToolService.ToggleMCPStatus(id); err != nil {
		c.RenderError(w, http.StatusInternalServerError, "切换工具状态失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:切换MCP工具状态", "operator", operator.Username, "id", id)

	c.Redirect(w, r, "/admin/mcp-tools")
}

// parseMCPID 从路由参数解析 ID
func (c *MCPToolCtrl) parseMCPID(r *http.Request) (uint, error) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

// renderMCPForm 渲染 MCP 工具表单（含错误信息）
func (c *MCPToolCtrl) renderMCPForm(w http.ResponseWriter, r *http.Request, title, action, errMsg string, t *model.MCPTool) {
	c.renderMCPFormWithJSON(w, r, title, action, errMsg, t, c.buildServerJSON(t))
}

// renderMCPFormWithJSON 渲染 MCP 工具表单（含错误信息和原始 JSON）
func (c *MCPToolCtrl) renderMCPFormWithJSON(w http.ResponseWriter, r *http.Request, title, action, errMsg string, t *model.MCPTool, serverJSON string) {
	data := map[string]interface{}{
		"Title":      title,
		"Error":      errMsg,
		"Tool":       t,
		"ServerJSON": serverJSON,
		"Action":     action,
		"AdminPage":  true,
		"ActiveMenu": "mcp-tools",
	}
	c.Render(w, r, "admin/mcp_tool_form", data)
}

// buildServerJSON 将 MCPTool 的 command/args/env 转换为美化的 MCPServerEntry JSON
func (c *MCPToolCtrl) buildServerJSON(t *model.MCPTool) string {
	if t == nil {
		return ""
	}
	var args []string
	if t.Args != "" {
		json.Unmarshal([]byte(t.Args), &args)
	}
	var env map[string]string
	if t.Env != "" {
		json.Unmarshal([]byte(t.Env), &env)
	}
	entry := model.MCPServerEntry{
		Command: t.Command,
		Args:    args,
		Env:     env,
	}
	b, _ := json.MarshalIndent(entry, "", "  ")
	return string(b)
}

// Import 批量导入 MCP 工具（支持文件上传或 JSON 文本）
func (c *MCPToolCtrl) Import(w http.ResponseWriter, r *http.Request) {
	var jsonData []byte

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		file, _, err := r.FormFile("file")
		if err != nil {
			common.JSONError(w, http.StatusBadRequest, "请选择 JSON 文件")
			return
		}
		defer file.Close()
		jsonData, err = io.ReadAll(file)
		if err != nil {
			common.JSONError(w, http.StatusBadRequest, "读取文件失败")
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			common.JSONError(w, http.StatusBadRequest, "解析请求失败")
			return
		}
		jsonStr := r.FormValue("json_content")
		if strings.TrimSpace(jsonStr) == "" {
			common.JSONError(w, http.StatusBadRequest, "请输入 mcpServers JSON 内容")
			return
		}
		jsonData = []byte(jsonStr)
	}

	var payload model.MCPServersPayload
	if err := json.Unmarshal(jsonData, &payload); err != nil {
		common.JSONError(w, http.StatusBadRequest, "JSON 格式错误: "+err.Error())
		return
	}
	if len(payload.MCPServers) == 0 {
		common.JSONError(w, http.StatusBadRequest, "未找到 mcpServers 配置项")
		return
	}

	reqs := make([]model.MCPToolCreateReq, 0, len(payload.MCPServers))
	for name, entry := range payload.MCPServers {
		argsJSON := ""
		if len(entry.Args) > 0 {
			if b, err := json.Marshal(entry.Args); err == nil {
				argsJSON = string(b)
			}
		}
		envJSON := ""
		if len(entry.Env) > 0 {
			if b, err := json.Marshal(entry.Env); err == nil {
				envJSON = string(b)
			}
		}
		reqs = append(reqs, model.MCPToolCreateReq{
			Name:    name,
			Label:   name,
			Command: entry.Command,
			Args:    argsJSON,
			Env:     envJSON,
			Timeout: 30,
		})
	}

	result, err := c.ToolService.ImportMCPTools(reqs)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "导入失败: "+err.Error())
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:批量导入MCP工具", "operator", operator.Username,
		"created", result.Created, "skipped", result.Skipped)

	common.JSONSuccess(w, result)
}

// Export 批量导出 MCP 工具为 mcpServers JSON 格式
func (c *MCPToolCtrl) Export(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := common.BindJSON(r, &req); err != nil || len(req.IDs) == 0 {
		common.JSONError(w, http.StatusBadRequest, "请选择要导出的工具")
		return
	}

	tools, err := c.ToolService.GetMCPToolsByIDs(req.IDs)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "查询工具失败")
		return
	}

	servers := make(map[string]model.MCPServerEntry, len(tools))
	for _, t := range tools {
		var args []string
		if t.Args != "" {
			if err := json.Unmarshal([]byte(t.Args), &args); err != nil {
				slog.Warn("导出 MCP 工具时 Args 解析失败", "name", t.Name, "error", err)
			}
		}
		var env map[string]string
		if t.Env != "" {
			if err := json.Unmarshal([]byte(t.Env), &env); err != nil {
				slog.Warn("导出 MCP 工具时 Env 解析失败", "name", t.Name, "error", err)
			}
		}
		servers[t.Name] = model.MCPServerEntry{
			Command: t.Command,
			Args:    args,
			Env:     env,
		}
	}

	output := model.MCPServersPayload{MCPServers: servers}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="mcp_servers_export.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

// Start 启动 MCP 服务器
func (c *MCPToolCtrl) Start(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseMCPID(r)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工具 ID")
		return
	}
	if err := c.ToolService.StartMCPServer(id); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "启动失败: "+err.Error())
		return
	}
	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:启动MCP服务器", "operator", operator.Username, "id", id)
	common.JSONSuccess(w, nil)
}

// Stop 停止 MCP 服务器
func (c *MCPToolCtrl) Stop(w http.ResponseWriter, r *http.Request) {
	id, err := c.parseMCPID(r)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工具 ID")
		return
	}
	if err := c.ToolService.StopMCPServer(id); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "停止失败: "+err.Error())
		return
	}
	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:停止MCP服务器", "operator", operator.Username, "id", id)
	common.JSONSuccess(w, nil)
}

// Statuses 获取所有 MCP 服务器运行状态
func (c *MCPToolCtrl) Statuses(w http.ResponseWriter, r *http.Request) {
	statuses := c.ToolService.GetMCPStatuses()
	common.JSONSuccess(w, statuses)
}
