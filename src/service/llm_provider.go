package service

import (
	"encoding/hex"
	"fmt"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/provider"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// LLMModelService LLM 模型业务逻辑（单表，替代原供应商+模型二层架构）
type LLMModelService struct {
	DB *gorm.DB
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

// GetByModelID 按 model_id 字符串查找
func (s *LLMModelService) GetByModelID(modelID string) (*model.LLMModel, error) {
	var m model.LLMModel
	if err := s.DB.Where("model_id = ?", modelID).First(&m).Error; err != nil {
		return nil, fmt.Errorf("模型不存在(model_id=%s): %w", modelID, err)
	}
	return &m, nil
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
		ReasoningEffort: req.ReasoningEffort,
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
	updates["reasoning_effort"] = req.ReasoningEffort

	if len(updates) > 0 {
		if err := s.DB.Model(m).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetByID(id)
}

// Delete 删除模型（软删除）
func (s *LLMModelService) Delete(id uint) error {
	return s.DB.Delete(&model.LLMModel{}, id).Error
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

// ImportModels 批量导入模型，model_id 已存在则跳过
func (s *LLMModelService) ImportModels(items []model.LLMModelCreateReq) (*ImportResult, error) {
	result := &ImportResult{}
	for _, req := range items {
		if err := common.ValidateStruct(&req); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", req.ModelID, err.Error()))
			continue
		}
		if _, err := s.GetByModelID(req.ModelID); err == nil {
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

// ========== 默认/自动切换 ==========

// SetDefault 将指定模型设为全局默认（清除其他 is_default，事务保证唯一性）
func (s *LLMModelService) SetDefault(id uint) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.LLMModel{}).Where("id != ?", id).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.LLMModel{}).Where("id = ?", id).Update("is_default", true).Error
	})
}

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

// GetDefault 返回 is_default=true 的模型
func (s *LLMModelService) GetDefault() (*model.LLMModel, error) {
	var m model.LLMModel
	if err := s.DB.Where("is_default = ?", true).First(&m).Error; err != nil {
		return nil, fmt.Errorf("未设置默认模型: %w", err)
	}
	return &m, nil
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
// "default" → is_default=true 的模型
// "auto"    → 第一个 is_auto=true 的模型（ChatRunner 迭代全列表时用 GetAutoModels）
// 其他      → 按 model_id 查找
func (s *LLMModelService) ResolveModel(modelID string) (*model.LLMModel, error) {
	switch modelID {
	case "default":
		return s.GetDefault()
	case "auto":
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
