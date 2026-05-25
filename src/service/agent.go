package service

import (
	"os"
	"path/filepath"
	"strings"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// AgentService AI 智能体业务逻辑
type AgentService struct {
	DB       *gorm.DB
	FileSync *FileSyncService
}

// NewAgentService 创建 AI 智能体服务实例
func NewAgentService(db *gorm.DB, fileSync *FileSyncService) *AgentService {
	return &AgentService{DB: db, FileSync: fileSync}
}

// List 获取 AI 智能体列表（分页）
func (s *AgentService) List(page, pageSize int) ([]model.Agent, int64, error) {
	var items []model.Agent
	var total int64

	query := s.DB.Model(&model.Agent{})

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Preload("Model").Offset(offset).Limit(pageSize).Order("sort ASC, id DESC").Find(&items).Error
	return items, total, err
}

// GetByID 根据 ID 获取 AI 智能体
func (s *AgentService) GetByID(id uint) (*model.Agent, error) {
	var item model.Agent
	err := s.DB.Preload("Model").First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetByName 根据名称获取 AI 智能体
func (s *AgentService) GetByName(name string) (*model.Agent, error) {
	var item model.Agent
	err := s.DB.Where("name = ?", name).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// GetAgentContent 读取智能体内容文件（供表单回显）
// 返回 map: role/memory/rule/skill/workflow → 文件内容
func (s *AgentService) GetAgentContent(name string) map[string]string {
	contextRoot := config.Get().Agent.ContextRoot
	dir := filepath.Join(contextRoot, "agents", name)
	result := map[string]string{
		"role":     readContentFile(filepath.Join(dir, "role.md")),
		"memory":   readContentFile(filepath.Join(dir, "memory.md")),
		"rule":     readContentFile(filepath.Join(dir, "rule.md")),
		"skill":    readContentFile(filepath.Join(dir, "skill.md")),
		"workflow": readContentFile(filepath.Join(dir, "workflow.md")),
	}
	return result
}

// Create 创建 AI 智能体
func (s *AgentService) Create(req *model.AgentCreateReq) (*model.Agent, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	agent := &model.Agent{
		Name:          req.Name,
		Title:         req.Title,
		Description:   req.Description,
		Keywords:      req.Keywords,
		ModelID:       req.ModelID,
		ModelMode:     req.ModelMode,
		AutoLoadFiles: req.AutoLoadFiles,
		AutoLoadTools: req.AutoLoadTools,
		Icon:          req.Icon,
		Status:        1,
	}

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(agent).Error; err != nil {
			return err
		}
		// 保存技能关联
		if len(req.SkillIDs) > 0 {
			for i, skillID := range req.SkillIDs {
				as := model.AgentSkill{AgentID: agent.ID, SkillID: skillID, Sort: i}
				if err := tx.Create(&as).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 同步元数据到文件系统
	if s.FileSync != nil {
		s.FileSync.SyncAgent(agent)
		// 写入内容文件
		go s.FileSync.WriteAgentContent(agent.Name, &req.RolePrompt, &req.MemoryContent, &req.RuleContent, &req.SkillContent, &req.WorkflowContent)
	}

	return agent, nil
}

// Update 更新 AI 智能体
func (s *AgentService) Update(id uint, req *model.AgentUpdateReq) (*model.Agent, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	agent, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	updates["description"] = req.Description
	updates["keywords"] = req.Keywords
	if req.ModelID != nil {
		updates["model_id"] = *req.ModelID
	}
	if req.ModelMode != nil {
		updates["model_mode"] = *req.ModelMode
	}
	if req.AutoLoadFiles != nil {
		updates["auto_load_files"] = *req.AutoLoadFiles
	}
	if req.AutoLoadTools != nil {
		updates["auto_load_tools"] = *req.AutoLoadTools
	}
	if req.Icon != nil {
		updates["icon"] = *req.Icon
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	txErr := s.DB.Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(agent).Updates(updates).Error; err != nil {
				return err
			}
		}
		// 更新技能关联
		if req.SkillIDs != nil {
			tx.Where("agent_id = ?", id).Delete(&model.AgentSkill{})
			for i, skillID := range req.SkillIDs {
				as := model.AgentSkill{AgentID: id, SkillID: skillID, Sort: i}
				if err := tx.Create(&as).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})

	if txErr != nil {
		return nil, txErr
	}

	result, err := s.GetByID(id)
	if err == nil && s.FileSync != nil {
		s.FileSync.SyncAgent(result)
		// 写入内容文件（nil 表示不更新该字段）
		go s.FileSync.WriteAgentContent(
			result.Name,
			req.RolePrompt,
			req.MemoryContent,
			req.RuleContent,
			req.SkillContent,
			req.WorkflowContent,
		)
	}
	return result, err
}

// Delete 删除 AI 智能体（软删除，同时清理技能关联）
func (s *AgentService) Delete(id uint) error {
	// 先获取名称用于删除文件
	agent, _ := s.GetByID(id)

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		tx.Where("agent_id = ?", id).Delete(&model.AgentSkill{})
		return tx.Delete(&model.Agent{}, id).Error
	})

	if err == nil && agent != nil && s.FileSync != nil {
		go s.FileSync.DeleteAgent(agent.Name)
	}
	return err
}

// Count 获取 AI 智能体总数
func (s *AgentService) Count() int64 {
	var count int64
	s.DB.Model(&model.Agent{}).Count(&count)
	return count
}

// ListAll 返回所有启用的 AI 智能体（下拉选择用）
func (s *AgentService) ListAll() ([]model.Agent, error) {
	var items []model.Agent
	err := s.DB.Where("status = ?", 1).Order("sort ASC, id ASC").Find(&items).Error
	return items, err
}

// GetSkillIDs 获取 AI 智能体的关联技能 ID 列表
func (s *AgentService) GetSkillIDs(agentID uint) []uint {
	var ids []uint
	s.DB.Model(&model.AgentSkill{}).Where("agent_id = ?", agentID).Order("sort ASC").Pluck("skill_id", &ids)
	return ids
}

// GetSkillNames 获取 AI 智能体关联技能名称，按关联顺序返回。
func (s *AgentService) GetSkillNames(agentID uint) []string {
	var names []string
	s.DB.Model(&model.Skill{}).
		Select("skills.name").
		Joins("JOIN agent_skills ON agent_skills.skill_id = skills.id").
		Where("agent_skills.agent_id = ?", agentID).
		Order("agent_skills.sort ASC, skills.sort ASC, skills.id ASC").
		Pluck("skills.name", &names)
	return names
}

// GetSkillsForAgent 获取运行时应注入的启用技能正文。
func (s *AgentService) GetSkillsForAgent(agentID uint) ([]model.Skill, error) {
	var skills []model.Skill
	err := s.DB.Model(&model.Skill{}).
		Select("skills.*").
		Joins("JOIN agent_skills ON agent_skills.skill_id = skills.id").
		Where("agent_skills.agent_id = ? AND skills.status = ?", agentID, 1).
		Order("agent_skills.sort ASC, skills.sort ASC, skills.id ASC").
		Find(&skills).Error
	return skills, err
}

// FindAgentByName 根据名称查找智能体，返回 (agentID, agentName, error)
// 实现 tool.AgentLookup 接口
func (s *AgentService) FindAgentByName(name string) (uint, string, error) {
	agent, err := s.GetByName(name)
	if err != nil {
		return 0, "", err
	}
	return agent.ID, agent.Name, nil
}

// ========== 工具接口方法（供 AI tool_call 调用） ==========

// WriteAgentFromTool 通过工具写入智能体：name 不存在则创建，已存在则更新
func (s *AgentService) WriteAgentFromTool(name, title, description, keywords, rolePrompt string, modelID string) (id uint, created bool, err error) {
	existing, _ := s.GetByName(name)
	if existing == nil {
		req := &model.AgentCreateReq{
			Name:        name,
			Title:       title,
			Description: description,
			Keywords:    keywords,
			RolePrompt:  rolePrompt,
			ModelID:     modelID,
		}
		agent, createErr := s.Create(req)
		if createErr != nil {
			return 0, false, createErr
		}
		return agent.ID, true, nil
	}

	req := &model.AgentUpdateReq{ModelID: &modelID, RolePrompt: &rolePrompt}
	if title != "" {
		req.Title = title
	}
	if description != "" {
		req.Description = description
	}
	if keywords != "" {
		req.Keywords = keywords
	}
	_, updateErr := s.Update(existing.ID, req)
	if updateErr != nil {
		return 0, false, updateErr
	}
	return existing.ID, false, nil
}

// ListAgentsForTool 为工具调用提供智能体列表（简化信息）
func (s *AgentService) ListAgentsForTool() ([]map[string]interface{}, error) {
	agents, err := s.ListAll()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, a := range agents {
		result = append(result, map[string]interface{}{
			"id":          a.ID,
			"name":        a.Name,
			"title":       a.Title,
			"description": a.Description,
		})
	}
	return result, nil
}

// ========== 内部工具函数 ==========

// readContentFile 读取内容文件，不存在返回空字符串
func readContentFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
