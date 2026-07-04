package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"agent_flow/src/model"

	"go.yaml.in/yaml/v3"
	"gorm.io/gorm"
)

// AgentSeedConfig agent.yaml 配置文件结构
type AgentSeedConfig struct {
	Name          string   `yaml:"name"`
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description"`
	Keywords      string   `yaml:"keywords"`
	ModelID       string   `yaml:"model_id"`
	ModelMode     string   `yaml:"model_mode"`
	Icon          string   `yaml:"icon"`
	Sort          int      `yaml:"sort"`
	Status        int      `yaml:"status"`
	AutoLoadFiles []string `yaml:"auto_load_files"`
	AutoLoadTools []string `yaml:"auto_load_tools"`
	DocRoles      string   `yaml:"doc_roles"`
	Skills        []string `yaml:"skills"`
}

// AgentSeedService 从文件系统扫描内置智能体并 upsert 到数据库
type AgentSeedService struct {
	db *gorm.DB
}

// NewAgentSeedService 创建 AgentSeedService
func NewAgentSeedService(db *gorm.DB) *AgentSeedService {
	return &AgentSeedService{db: db}
}

// Seed 扫描 {contextRoot}/agents/ 目录，将所有内置智能体 upsert 到数据库
func (s *AgentSeedService) Seed(contextRoot string) error {
	agentsDir := filepath.Join(contextRoot, "agents")

	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		slog.Debug("内置智能体目录不存在，跳过加载", "dir", agentsDir)
		return nil
	}

	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return fmt.Errorf("读取智能体目录失败: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		agentDir := filepath.Join(agentsDir, entry.Name())
		yamlPath := filepath.Join(agentDir, "agent.yaml")

		// agent.yaml 必须存在
		if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
			continue
		}

		cfg, err := s.loadConfig(yamlPath)
		if err != nil {
			slog.Warn("解析 agent.yaml 失败", "dir", agentDir, "error", err)
			continue
		}

		if err := s.upsert(cfg); err != nil {
			slog.Warn("内置智能体 upsert 失败", "name", cfg.Name, "error", err)
			continue
		}

		count++
	}

	slog.Info("内置智能体加载完成", "count", count)
	return nil
}

// loadConfig 读取并解析 agent.yaml
func (s *AgentSeedService) loadConfig(yamlPath string) (*AgentSeedConfig, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("读取 agent.yaml 失败: %w", err)
	}

	var cfg AgentSeedConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析 agent.yaml 失败: %w", err)
	}

	if cfg.Name == "" {
		return nil, fmt.Errorf("agent.yaml 缺少 name 字段")
	}

	return &cfg, nil
}

// upsert 将智能体配置写入数据库（不存在则创建，已存在则更新元数据）
func (s *AgentSeedService) upsert(cfg *AgentSeedConfig) error {
	autoLoadFiles, err := json.Marshal(cfg.AutoLoadFiles)
	if err != nil {
		return fmt.Errorf("序列化 auto_load_files 失败: %w", err)
	}
	autoLoadTools, err := json.Marshal(cfg.AutoLoadTools)
	if err != nil {
		return fmt.Errorf("序列化 auto_load_tools 失败: %w", err)
	}

	status := cfg.Status
	if status == 0 {
		status = 1
	}

	modelID := cfg.ModelID
	if modelID == "" {
		modelID = "auto"
	}
	modelMode := cfg.ModelMode
	if modelMode == "" {
		modelMode = "auto"
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.Agent
		result := tx.Where("name = ?", cfg.Name).First(&existing)

		var agentID uint

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			agent := model.Agent{
				Name:          cfg.Name,
				Title:         cfg.Title,
				Description:   cfg.Description,
				Keywords:      cfg.Keywords,
				ModelID:       modelID,
				ModelMode:     modelMode,
				Icon:          cfg.Icon,
				Sort:          cfg.Sort,
				Status:        status,
				AutoLoadFiles: string(autoLoadFiles),
				AutoLoadTools: string(autoLoadTools),
				DocRoles:      cfg.DocRoles,
			}
			if err := tx.Create(&agent).Error; err != nil {
				return fmt.Errorf("创建智能体失败: %w", err)
			}
			agentID = agent.ID
			slog.Info("内置智能体已创建", "name", cfg.Name, "title", cfg.Title)
		} else if result.Error == nil {
			updates := map[string]interface{}{
				"title":           cfg.Title,
				"description":     cfg.Description,
				"keywords":        cfg.Keywords,
				"icon":            cfg.Icon,
				"model_id":        modelID,
				"model_mode":      modelMode,
				"sort":            cfg.Sort,
				"status":          status,
				"auto_load_files": string(autoLoadFiles),
				"auto_load_tools": string(autoLoadTools),
				"doc_roles":       cfg.DocRoles,
			}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return fmt.Errorf("更新智能体失败: %w", err)
			}
			agentID = existing.ID
			slog.Debug("内置智能体已更新", "name", cfg.Name)
		} else {
			return fmt.Errorf("查询智能体失败: %w", result.Error)
		}

		return s.syncAgentSkills(tx, agentID, cfg)
	})
}

func (s *AgentSeedService) syncAgentSkills(tx *gorm.DB, agentID uint, cfg *AgentSeedConfig) error {
	if err := tx.Where("agent_id = ?", agentID).Delete(&model.AgentSkill{}).Error; err != nil {
		return fmt.Errorf("清理智能体技能关联失败: %w", err)
	}

	seen := make(map[string]bool, len(cfg.Skills))
	sort := 0
	for _, skillName := range cfg.Skills {
		if skillName == "" || seen[skillName] {
			continue
		}
		seen[skillName] = true

		var skill model.Skill
		if err := tx.Where("name = ?", skillName).First(&skill).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				slog.Warn("agent.yaml references missing skill", "agent", cfg.Name, "skill", skillName)
				continue
			}
			return fmt.Errorf("查询技能 %q 失败: %w", skillName, err)
		}

		if err := tx.Create(&model.AgentSkill{AgentID: agentID, SkillID: skill.ID, Sort: sort}).Error; err != nil {
			return fmt.Errorf("创建智能体技能关联 %q 失败: %w", skillName, err)
		}
		sort++
	}

	return nil
}
