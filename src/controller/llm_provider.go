package controller

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/middleware"
	"agent_flow/src/model"
	"agent_flow/src/provider"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// LLMProviderCtrl 模型管理控制器（路由名保持兼容，实际管理 llm_models 单表）
type LLMProviderCtrl struct {
	Base
	Service *service.LLMModelService
}

// List 模型列表页面
func (c *LLMProviderCtrl) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize := config.Get().Pagination.DefaultPageSize

	models, total, err := c.Service.List(page, pageSize)
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取模型列表失败")
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
		"Title":      "模型管理",
		"Models":     models,
		"Paging":     paging,
		"AdminPage":  true,
		"ActiveMenu": "models",
	}
	c.Render(w, r, "admin/llm_model_list", data)
}

// CreateForm 创建模型表单页面
func (c *LLMProviderCtrl) CreateForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":      "添加模型",
		"Action":     "/admin/models",
		"Method":     "POST",
		"FormModel":  model.LLMModel{MaxInputTokens: 200000, MaxOutputTokens: 128000, Capabilities: "tools"},
		"HasVision":  false,
		"HasImageGen": false,
		"HasTools":   true,
		"AdminPage":  true,
		"ActiveMenu": "models",
	}
	c.Render(w, r, "admin/llm_model_form", data)
}

// Create 创建模型
func (c *LLMProviderCtrl) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	maxIn, _ := strconv.Atoi(r.FormValue("max_input_tokens"))
	maxOut, _ := strconv.Atoi(r.FormValue("max_output_tokens"))

	req := &model.LLMModelCreateReq{
		ModelID:         r.FormValue("model_id"),
		Name:            r.FormValue("name"),
		BaseURL:         r.FormValue("base_url"),
		Protocol:        r.FormValue("protocol"),
		APIKey:          r.FormValue("api_key"),
		APIModelID:      r.FormValue("api_model_id"),
		MaxInputTokens:  maxIn,
		MaxOutputTokens: maxOut,
		IsAuto:          r.FormValue("is_auto") == "1",
		Capabilities:    buildCapabilities(r),
		ReasoningEffort: r.FormValue("reasoning_effort"),
	}

	m, err := c.Service.Create(req)
	if err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "创建模型失败: "+err.Error())
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:创建模型", "operator", operator.Username, "model_id", m.ModelID)

	c.Redirect(w, r, "/admin/models")
}

// EditForm 编辑模型表单页面
func (c *LLMProviderCtrl) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	m, err := c.Service.GetByID(uint(id))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "模型不存在")
		return
	}

	data := map[string]interface{}{
		"Title":       "编辑模型",
		"Action":      "/admin/models/" + chi.URLParam(r, "id"),
		"Method":      "PUT",
		"FormModel":   m,
		"HasVision":   m.HasCapability("vision"),
		"HasImageGen": m.HasCapability("image_gen"),
		"HasTools":    m.HasCapability("tools"),
		"AdminPage":   true,
		"ActiveMenu":  "models",
	}
	c.Render(w, r, "admin/llm_model_form", data)
}

// Update 更新模型
func (c *LLMProviderCtrl) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	maxIn, _ := strconv.Atoi(r.FormValue("max_input_tokens"))
	maxOut, _ := strconv.Atoi(r.FormValue("max_output_tokens"))
	isAuto := r.FormValue("is_auto") == "1"

	req := &model.LLMModelUpdateReq{
		Name:            r.FormValue("name"),
		BaseURL:         r.FormValue("base_url"),
		Protocol:        r.FormValue("protocol"),
		APIKey:          r.FormValue("api_key"),
		APIModelID:      r.FormValue("api_model_id"),
		MaxInputTokens:  maxIn,
		MaxOutputTokens: maxOut,
		IsAuto:          &isAuto,
		Capabilities:    buildCapabilities(r),
		ReasoningEffort: r.FormValue("reasoning_effort"),
	}

	if _, err := c.Service.Update(uint(id), req); err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			c.RenderError(w, appErr.Code, appErr.Message)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "更新模型失败")
		return
	}

	c.Redirect(w, r, "/admin/models")
}

// Delete 删除模型
func (c *LLMProviderCtrl) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	m, err := c.Service.GetByID(uint(id))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "模型不存在")
		return
	}

	if err := c.Service.Delete(uint(id)); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "删除模型失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:删除模型", "operator", operator.Username, "model_id", m.ModelID)

	common.JSONSuccess(w, nil)
}

// ToggleAuto 切换模型的自动切换参与状态
func (c *LLMProviderCtrl) ToggleAuto(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	newVal, err := c.Service.ToggleAuto(uint(id))
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "更新失败")
		return
	}

	common.JSONSuccess(w, map[string]interface{}{"id": id, "is_auto": newVal})
}

// Export 批量导出模型为 JSON 文件下载
func (c *LLMProviderCtrl) Export(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uint `json:"ids"`
	}
	if err := common.BindJSON(r, &req); err != nil || len(req.IDs) == 0 {
		common.JSONError(w, http.StatusBadRequest, "请选择要导出的模型")
		return
	}

	models, err := c.Service.GetByIDs(req.IDs)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "查询模型失败")
		return
	}

	exports := make([]model.LLMModelExport, 0, len(models))
	for i := range models {
		exports = append(exports, model.LLMModelExport{
			ModelID:         models[i].ModelID,
			Name:            models[i].Name,
			BaseURL:         models[i].BaseURL,
			Protocol:        models[i].Protocol,
			APIKey:          c.Service.DecryptAPIKey(&models[i]),
			APIModelID:      models[i].APIModelID,
			MaxInputTokens:  models[i].MaxInputTokens,
			MaxOutputTokens: models[i].MaxOutputTokens,
			IsAuto:          models[i].IsAuto,
			Capabilities:    models[i].Capabilities,
			ReasoningEffort: models[i].ReasoningEffort,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="llm_models_export.json"`)
	json.NewEncoder(w).Encode(exports)
}

// Import 从 JSON 文件批量导入模型
func (c *LLMProviderCtrl) Import(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		common.JSONError(w, http.StatusBadRequest, "文件解析失败")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "请选择要导入的文件")
		return
	}
	defer file.Close()

	var items []model.LLMModelExport
	if err := json.NewDecoder(file).Decode(&items); err != nil {
		common.JSONError(w, http.StatusBadRequest, "JSON 格式错误: "+err.Error())
		return
	}
	if len(items) == 0 {
		common.JSONError(w, http.StatusBadRequest, "文件中没有模型数据")
		return
	}

	reqs := make([]model.LLMModelCreateReq, 0, len(items))
	for _, item := range items {
		reqs = append(reqs, model.LLMModelCreateReq{
			ModelID:         item.ModelID,
			Name:            item.Name,
			BaseURL:         item.BaseURL,
			Protocol:        item.Protocol,
			APIKey:          item.APIKey,
			APIModelID:      item.APIModelID,
			MaxInputTokens:  item.MaxInputTokens,
			MaxOutputTokens: item.MaxOutputTokens,
			IsAuto:          item.IsAuto,
			Capabilities:    item.Capabilities,
			ReasoningEffort: item.ReasoningEffort,
		})
	}

	result, err := c.Service.ImportModels(reqs)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "导入失败: "+err.Error())
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:批量导入模型", "operator", operator.Username,
		"created", result.Created, "skipped", result.Skipped)

	common.JSONSuccess(w, result)
}

// TestConnect 测试模型 API 连通性
func (c *LLMProviderCtrl) TestConnect(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	m, err := c.Service.GetByID(uint(id))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "模型不存在")
		return
	}

	apiKey := c.Service.DecryptAPIKey(m)
	client, err := provider.NewClientByProtocol(m.Protocol, provider.ProviderConfig{
		BaseURL: m.BaseURL,
		APIKey:  apiKey,
		Timeout: config.Get().LLM.Timeout,
	})
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "创建客户端失败: "+err.Error())
		return
	}

	content, err := provider.QuickTest(r.Context(), client, m.APIModelID)
	if err != nil {
		common.JSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	common.JSONSuccess(w, map[string]string{"status": "ok", "content": content})
}

// FetchModels 获取上游可用模型列表
func (c *LLMProviderCtrl) FetchModels(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	var baseURL, apiKey, protocol string

	if idStr != "" {
		// 从已有模型读取配置
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			common.JSONError(w, http.StatusBadRequest, "无效的 ID")
			return
		}
		m, err := c.Service.GetByID(uint(id))
		if err != nil {
			common.JSONError(w, http.StatusNotFound, "模型不存在")
			return
		}
		baseURL = m.BaseURL
		apiKey = c.Service.DecryptAPIKey(m)
		protocol = m.Protocol
	} else {
		// 从请求体读取配置
		var req struct {
			BaseURL  string `json:"base_url"`
			APIKey   string `json:"api_key"`
			Protocol string `json:"protocol"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.JSONError(w, http.StatusBadRequest, "参数解析失败")
			return
		}
		if req.BaseURL == "" || req.APIKey == "" || req.Protocol == "" {
			common.JSONError(w, http.StatusBadRequest, "base_url、api_key、protocol 不能为空")
			return
		}
		baseURL = req.BaseURL
		apiKey = req.APIKey
		protocol = req.Protocol
	}

	models, err := c.Service.FetchUpstreamModels(baseURL, apiKey, protocol)
	if err != nil {
		common.JSONError(w, http.StatusBadGateway, "获取上游模型失败: "+err.Error())
		return
	}

	common.JSONSuccess(w, models)
}

// buildCapabilities 从表单 checkbox 构建逗号分隔的能力字符串
func buildCapabilities(r *http.Request) string {
	var caps []string
	if r.FormValue("cap_vision") == "1" {
		caps = append(caps, "vision")
	}
	if r.FormValue("cap_image_gen") == "1" {
		caps = append(caps, "image_gen")
	}
	if r.FormValue("cap_tools") == "1" {
		caps = append(caps, "tools")
	}
	if len(caps) == 0 {
		return "tools"
	}
	return strings.Join(caps, ",")
}
