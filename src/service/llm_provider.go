package service

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/provider"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// LLMModelService LLM 模型业务逻辑（单表，替代原供应商+模型二层架构）
type LLMModelService struct {
	DB               *gorm.DB
	SysConfigService *SysConfigService
}

// NewLLMProviderService 创建服务实例（保持原函数名兼容路由注册）
func NewLLMProviderService(db *gorm.DB) *LLMModelService {
	return &LLMModelService{DB: db}
}

// ========== CRUD ==========

// List 获取模型分页列表
func (s *LLMModelService) List(page, pageSize int) ([]model.LLMModel, int64, error) {
	var items []model.LLMModel
	var total int64

	s.DB.Model(&model.LLMModel{}).Count(&total)

	offset := (page - 1) * pageSize
	err := s.DB.Offset(offset).Limit(pageSize).Order("id ASC").Find(&items).Error
	return items, total, err
}

// ListAll 返回所有模型（不分页，供下拉选择）
func (s *LLMModelService) ListAll() ([]model.LLMModel, error) {
	var items []model.LLMModel
	err := s.DB.Order("id ASC").Find(&items).Error
	return items, err
}

// GetByID 根据主键获取模型
func (s *LLMModelService) GetByID(id uint) (*model.LLMModel, error) {
	var m model.LLMModel
	if err := s.DB.First(&m, id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// GetByModelID 按 model_id 字符串查找（返回第一条匹配记录）
func (s *LLMModelService) GetByModelID(modelID string) (*model.LLMModel, error) {
	var m model.LLMModel
	if err := s.DB.Where("model_id = ?", modelID).First(&m).Error; err != nil {
		return nil, fmt.Errorf("模型不存在(model_id=%s): %w", modelID, err)
	}
	return &m, nil
}

// GetAllByModelID 按 model_id 查找所有匹配记录（负载均衡场景）
func (s *LLMModelService) GetAllByModelID(modelID string) ([]model.LLMModel, error) {
	var items []model.LLMModel
	err := s.DB.Where("model_id = ?", modelID).Order("id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("模型不存在(model_id=%s)", modelID)
	}
	return items, nil
}

// Create 创建模型（APIKey AES 加密存储）
func (s *LLMModelService) Create(req *model.LLMModelCreateReq) (*model.LLMModel, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	m := &model.LLMModel{
		ModelID:         req.ModelID,
		Name:            req.Name,
		BaseURL:         req.BaseURL,
		Protocol:        req.Protocol,
		APIModelID:      req.APIModelID,
		MaxInputTokens:  req.MaxInputTokens,
		MaxOutputTokens: req.MaxOutputTokens,
		IsAuto:          req.IsAuto,
		Capabilities:    req.Capabilities,
		ReasoningEffort: req.ReasoningEffort,
	}

	if m.Capabilities == "" {
		m.Capabilities = "tools"
	}

	if m.MaxInputTokens == 0 {
		m.MaxInputTokens = 128000
	}
	if m.MaxOutputTokens == 0 {
		m.MaxOutputTokens = 4096
	}

	m.APIKey = s.encryptKey(req.APIKey)

	if err := s.DB.Create(m).Error; err != nil {
		return nil, err
	}
	return m, nil
}

// Update 更新模型（APIKey 留空时不更新）
func (s *LLMModelService) Update(id uint, req *model.LLMModelUpdateReq) (*model.LLMModel, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	m, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.BaseURL != "" {
		updates["base_url"] = req.BaseURL
	}
	if req.Protocol != "" {
		updates["protocol"] = req.Protocol
	}
	if req.APIKey != "" {
		updates["api_key"] = s.encryptKey(req.APIKey)
	}
	if req.APIModelID != "" {
		updates["api_model_id"] = req.APIModelID
	}
	if req.MaxInputTokens > 0 {
		updates["max_input_tokens"] = req.MaxInputTokens
	}
	if req.MaxOutputTokens > 0 {
		updates["max_output_tokens"] = req.MaxOutputTokens
	}
	if req.IsAuto != nil {
		updates["is_auto"] = *req.IsAuto
	}
	updates["capabilities"] = req.Capabilities
	updates["reasoning_effort"] = req.ReasoningEffort

	if len(updates) > 0 {
		if err := s.DB.Model(m).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetByID(id)
}

// Delete 删除模型
func (s *LLMModelService) Delete(id uint) error {
	return s.DB.Delete(&model.LLMModel{}, id).Error
}

// ListByCapability 查询含指定能力的模型
func (s *LLMModelService) ListByCapability(cap string) ([]model.LLMModel, error) {
	var items []model.LLMModel
	err := s.DB.Where("capabilities LIKE ?", "%"+cap+"%").Order("id ASC").Find(&items).Error
	return items, err
}

// GetByIDs 按主键批量查询模型
func (s *LLMModelService) GetByIDs(ids []uint) ([]model.LLMModel, error) {
	var items []model.LLMModel
	if len(ids) == 0 {
		return items, nil
	}
	err := s.DB.Where("id IN ?", ids).Order("id ASC").Find(&items).Error
	return items, err
}

// ImportResult 批量导入结果
type ImportResult struct {
	Created    int      `json:"created"`
	Skipped    int      `json:"skipped"`
	SkippedIDs []string `json:"skipped_ids,omitempty"`
	Errors     []string `json:"errors,omitempty"`
}

// ImportModels 批量导入模型，model_id + base_url 组合已存在则跳过
func (s *LLMModelService) ImportModels(items []model.LLMModelCreateReq) (*ImportResult, error) {
	result := &ImportResult{}
	for _, req := range items {
		if err := common.ValidateStruct(&req); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", req.ModelID, err.Error()))
			continue
		}
		var count int64
		s.DB.Model(&model.LLMModel{}).Where("model_id = ? AND base_url = ?", req.ModelID, req.BaseURL).Count(&count)
		if count > 0 {
			result.Skipped++
			result.SkippedIDs = append(result.SkippedIDs, req.ModelID)
			continue
		}
		reqCopy := req
		if _, err := s.Create(&reqCopy); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", req.ModelID, err.Error()))
			continue
		}
		result.Created++
	}
	return result, nil
}

// ========== 自动切换 ==========

// ToggleAuto 翻转 is_auto 标志
func (s *LLMModelService) ToggleAuto(id uint) (bool, error) {
	m, err := s.GetByID(id)
	if err != nil {
		return false, err
	}
	newVal := !m.IsAuto
	if err := s.DB.Model(m).Update("is_auto", newVal).Error; err != nil {
		return false, err
	}
	return newVal, nil
}

// GetAutoModels 返回所有 is_auto=true 的模型，按 id 排序
func (s *LLMModelService) GetAutoModels() ([]model.LLMModel, error) {
	var items []model.LLMModel
	err := s.DB.Where("is_auto = ?", true).Order("id ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("没有配置自动切换模型（请在模型管理中开启 is_auto）")
	}
	return items, nil
}

// ResolveModel 解析 modelID 到具体模型记录
// "auto"/"default" → 从 is_auto=true 的模型中取第一个（ChatRunner 迭代全列表时用 GetAutoModels）
// 其他             → 按 model_id 查找
func (s *LLMModelService) ResolveModel(modelID string) (*model.LLMModel, error) {
	switch modelID {
	case "auto", "default":
		models, err := s.GetAutoModels()
		if err != nil {
			return nil, err
		}
		return &models[0], nil
	default:
		return s.GetByModelID(modelID)
	}
}

// ========== 客户端创建 ==========

// DecryptAPIKey 解密模型的 API Key
func (s *LLMModelService) DecryptAPIKey(m *model.LLMModel) string {
	secretKey := config.Get().App.SecretKey
	if secretKey == "" {
		return m.APIKey
	}
	key, err := hex.DecodeString(secretKey)
	if err != nil {
		return m.APIKey
	}
	decrypted, err := common.DecryptAES(m.APIKey, key)
	if err != nil {
		return m.APIKey
	}
	return decrypted
}

// CreateClientFromModel 根据模型配置创建 LLM 客户端
func (s *LLMModelService) CreateClientFromModel(m *model.LLMModel) (provider.Client, error) {
	apiKey := s.DecryptAPIKey(m)
	llmCfg := config.Get().LLM
	return provider.NewClientByProtocol(m.Protocol, provider.ProviderConfig{
		BaseURL:    m.BaseURL,
		APIKey:     apiKey,
		Timeout:    llmCfg.Timeout,
		MaxRetries: llmCfg.MaxRetries,
	})
}

// Count 获取模型总数
func (s *LLMModelService) Count() int64 {
	var count int64
	s.DB.Model(&model.LLMModel{}).Count(&count)
	return count
}

// ========== 内部辅助 ==========

func (s *LLMModelService) encryptKey(raw string) string {
	secretKey := config.Get().App.SecretKey
	if secretKey == "" {
		return raw
	}
	key, err := hex.DecodeString(secretKey)
	if err != nil {
		return raw
	}
	encrypted, err := common.EncryptAES(raw, key)
	if err != nil {
		return raw
	}
	return encrypted
}

// ========== 上游模型获取 ==========

// UpstreamModel 上游模型信息
type UpstreamModel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	MaxInputTokens  int    `json:"max_input_tokens"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	Capabilities    string `json:"capabilities"`
}

// FetchUpstreamModels 从上游 API 获取可用模型列表
func (s *LLMModelService) FetchUpstreamModels(baseURL, apiKey, protocol string) ([]UpstreamModel, error) {
	switch protocol {
	case "anthropic":
		return s.fetchAnthropicModels(baseURL, apiKey)
	case "gemini":
		return s.fetchGeminiModels(baseURL, apiKey)
	default:
		return s.fetchOpenAIModels(baseURL, apiKey)
	}
}

func (s *LLMModelService) fetchOpenAIModels(baseURL, apiKey string) ([]UpstreamModel, error) {
	url := common.BuildAPIURL(baseURL, "/models")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("API 错误 %d: %s", resp.StatusCode, preview)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	models := make([]UpstreamModel, 0, len(result.Data))
	for _, item := range result.Data {
		m := UpstreamModel{
			ID:              item.ID,
			Name:            item.ID,
			MaxInputTokens:  inferMaxInputTokens(item.ID),
			MaxOutputTokens: inferMaxOutputTokens(item.ID),
			Capabilities:    inferCapabilities(item.ID),
		}
		models = append(models, m)
	}
	return models, nil
}

func (s *LLMModelService) fetchAnthropicModels(baseURL, apiKey string) ([]UpstreamModel, error) {
	url := common.BuildAPIURL(baseURL, "/v1/models")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("API 错误 %d: %s", resp.StatusCode, preview)
	}

	var result struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	models := make([]UpstreamModel, 0, len(result.Data))
	for _, item := range result.Data {
		name := item.DisplayName
		if name == "" {
			name = item.ID
		}
		m := UpstreamModel{
			ID:              item.ID,
			Name:            name,
			MaxInputTokens:  inferMaxInputTokens(item.ID),
			MaxOutputTokens: inferMaxOutputTokens(item.ID),
			Capabilities:    inferCapabilities(item.ID),
		}
		models = append(models, m)
	}
	return models, nil
}

func (s *LLMModelService) fetchGeminiModels(baseURL, apiKey string) ([]UpstreamModel, error) {
	url := common.BuildAPIURL(baseURL, "/v1beta/models") + "?key=" + apiKey
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("API 错误 %d: %s", resp.StatusCode, preview)
	}

	var result struct {
		Models []struct {
			Name                string `json:"name"`
			DisplayName         string `json:"displayName"`
			InputTokenLimit     int    `json:"inputTokenLimit"`
			OutputTokenLimit    int    `json:"outputTokenLimit"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	models := make([]UpstreamModel, 0, len(result.Models))
	for _, item := range result.Models {
		// Gemini model name format: "models/gemini-1.5-pro"
		id := strings.TrimPrefix(item.Name, "models/")
		name := item.DisplayName
		if name == "" {
			name = id
		}
		maxIn := item.InputTokenLimit
		if maxIn == 0 {
			maxIn = inferMaxInputTokens(id)
		}
		maxOut := item.OutputTokenLimit
		if maxOut == 0 {
			maxOut = inferMaxOutputTokens(id)
		}
		m := UpstreamModel{
			ID:              id,
			Name:            name,
			MaxInputTokens:  maxIn,
			MaxOutputTokens: maxOut,
			Capabilities:    inferCapabilities(id),
		}
		models = append(models, m)
	}
	return models, nil
}

// inferMaxInputTokens 根据模型 ID 推断最大输入 token
func inferMaxInputTokens(modelID string) int {
	id := strings.ToLower(modelID)
	switch {
	case strings.Contains(id, "gpt-4o"), strings.Contains(id, "gpt-4-turbo"):
		return 128000
	case strings.Contains(id, "gpt-4"):
		return 8192
	case strings.Contains(id, "gpt-3.5"):
		return 16385
	case strings.Contains(id, "claude-3"), strings.Contains(id, "claude-sonnet"), strings.Contains(id, "claude-opus"), strings.Contains(id, "claude-haiku"):
		return 200000
	case strings.Contains(id, "gemini-1.5"), strings.Contains(id, "gemini-2"):
		return 1048576
	case strings.Contains(id, "gemini"):
		return 32768
	case strings.Contains(id, "deepseek"):
		return 128000
	case strings.Contains(id, "qwen"):
		return 131072
	default:
		return 128000
	}
}

// inferMaxOutputTokens 根据模型 ID 推断最大输出 token
func inferMaxOutputTokens(modelID string) int {
	id := strings.ToLower(modelID)
	switch {
	case strings.Contains(id, "gpt-4o"):
		return 16384
	case strings.Contains(id, "gpt-4"):
		return 4096
	case strings.Contains(id, "gpt-3.5"):
		return 4096
	case strings.Contains(id, "claude-3-5"), strings.Contains(id, "claude-sonnet-4"), strings.Contains(id, "claude-opus"):
		return 8192
	case strings.Contains(id, "claude"):
		return 4096
	case strings.Contains(id, "gemini"):
		return 8192
	case strings.Contains(id, "deepseek"):
		return 8192
	default:
		return 4096
	}
}

// inferCapabilities 根据模型 ID 推断能力标记
func inferCapabilities(modelID string) string {
	id := strings.ToLower(modelID)
	caps := []string{"tools"}
	switch {
	case strings.Contains(id, "gpt-4o"), strings.Contains(id, "gpt-4-turbo"), strings.Contains(id, "gpt-4-vision"):
		caps = append(caps, "vision")
	case strings.Contains(id, "claude-3"), strings.Contains(id, "claude-sonnet"), strings.Contains(id, "claude-opus"), strings.Contains(id, "claude-haiku"):
		caps = append(caps, "vision")
	case strings.Contains(id, "gemini"):
		caps = append(caps, "vision")
	case strings.Contains(id, "qwen-vl"), strings.Contains(id, "qwen2-vl"):
		caps = append(caps, "vision")
	case strings.Contains(id, "dall-e"), strings.Contains(id, "imagen"):
		caps = append(caps, "image_gen")
	}
	return strings.Join(caps, ",")
}

// ========== 图片生成模型获取 ==========

// GetModelForImageGen 实现 tool.ModelGetter 接口，获取图片生成模型配置
func (s *LLMModelService) GetModelForImageGen() (baseURL, apiKey, apiModelID, protocol string, err error) {
	if s.SysConfigService == nil {
		return "", "", "", "", fmt.Errorf("系统配置服务未初始化")
	}

	modelID := s.SysConfigService.Get("ai.image_gen_model")
	if modelID == "" {
		return "", "", "", "", fmt.Errorf("未配置图片生成模型（系统配置 ai.image_gen_model）")
	}

	m, err := s.GetByModelID(modelID)
	if err != nil {
		return "", "", "", "", fmt.Errorf("图片生成模型不存在: %w", err)
	}

	decryptedKey := s.DecryptAPIKey(m)
	return m.BaseURL, decryptedKey, m.APIModelID, m.Protocol, nil
}
