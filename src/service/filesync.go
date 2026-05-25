package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"agent_flow/src/config"
	"agent_flow/src/model"

	"go.yaml.in/yaml/v3"
	"gorm.io/gorm"
)

// FileSyncService 文件系统同步服务（数据库 → 文件系统）
type FileSyncService struct {
	DB *gorm.DB
}

// NewFileSyncService 创建文件同步服务实例
func NewFileSyncService(db *gorm.DB) *FileSyncService {
	return &FileSyncService{DB: db}
}

// ---- 智能体同步 ----

// agentConfig 智能体 agent.yaml 结构（元数据，不含内容字段）
type agentConfig struct {
	Name          string   `yaml:"name"`
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description,omitempty"`
	Keywords      string   `yaml:"keywords,omitempty"`
	Icon          string   `yaml:"icon,omitempty"`
	ModelID       string   `yaml:"model_id"`
	ModelMode     string   `yaml:"model_mode"`
	Sort          int      `yaml:"sort"`
	Status        int      `yaml:"status"`
	AutoLoadFiles []string `yaml:"auto_load_files"`
	AutoLoadTools []string `yaml:"auto_load_tools"`
	Skills        []string `yaml:"skills,omitempty"`
}

// SyncAgent 同步智能体元数据到文件系统（agent.yaml）
// 内容文件（role.md 等）由 WriteAgentContent 单独写入
func (s *FileSyncService) SyncAgent(agent *model.Agent) {
	contextRoot := config.Get().Agent.ContextRoot
	dir := filepath.Join(contextRoot, "agents", agent.Name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("创建智能体目录失败", "name", agent.Name, "error", err)
		return
	}

	cfg := agentConfig{
		Name:          agent.Name,
		Title:         agent.Title,
		Description:   agent.Description,
		Keywords:      agent.Keywords,
		Icon:          agent.Icon,
		ModelID:       agent.ModelID,
		ModelMode:     agent.ModelMode,
		Sort:          agent.Sort,
		Status:        agent.Status,
		AutoLoadFiles: parseJSONStringArray(agent.AutoLoadFiles),
		AutoLoadTools: parseJSONStringArray(agent.AutoLoadTools),
		Skills:        s.getAgentSkillNames(agent.ID),
	}
	writeYAML(filepath.Join(dir, "agent.yaml"), cfg)

	slog.Info("智能体元数据同步完成", "name", agent.Name, "dir", dir)
}

func (s *FileSyncService) getAgentSkillNames(agentID uint) []string {
	var names []string
	s.DB.Model(&model.Skill{}).
		Select("skills.name").
		Joins("JOIN agent_skills ON agent_skills.skill_id = skills.id").
		Where("agent_skills.agent_id = ?", agentID).
		Order("agent_skills.sort ASC, skills.sort ASC, skills.id ASC").
		Pluck("skills.name", &names)
	return names
}

// WriteAgentContent 将智能体内容字段写入文件（context/agents/{name}/*.md）
// 指针语义：nil=不操作，""=删除文件，非空=写入文件
func (s *FileSyncService) WriteAgentContent(name string, rolePrompt, memoryContent, ruleContent, skillContent, workflowContent *string) {
	contextRoot := config.Get().Agent.ContextRoot
	dir := filepath.Join(contextRoot, "agents", name)

	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("创建智能体目录失败", "name", name, "error", err)
		return
	}

	syncContentFile(filepath.Join(dir, "role.md"), rolePrompt)
	syncContentFile(filepath.Join(dir, "memory.md"), memoryContent)
	syncContentFile(filepath.Join(dir, "rule.md"), ruleContent)
	syncContentFile(filepath.Join(dir, "skill.md"), skillContent)
	syncContentFile(filepath.Join(dir, "workflow.md"), workflowContent)
}

// DeleteAgent 删除智能体文件目录
func (s *FileSyncService) DeleteAgent(name string) {
	contextRoot := config.Get().Agent.ContextRoot
	dir := filepath.Join(contextRoot, "agents", name)
	if err := os.RemoveAll(dir); err != nil {
		slog.Error("删除智能体目录失败", "name", name, "error", err)
		return
	}
	slog.Info("智能体文件目录已删除", "name", name)
}

// ---- 技能同步 ----

// SyncSkill 同步技能数据到文件系统（YAML front matter + content）
func (s *FileSyncService) SyncSkill(skill *model.Skill) {
	contextRoot := config.Get().Agent.ContextRoot
	dir := filepath.Join(contextRoot, "skills")

	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("创建技能目录失败", "error", err)
		return
	}

	// 构建 YAML front matter + content
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", skill.Name))
	sb.WriteString(fmt.Sprintf("label: %s\n", skill.Label))
	if skill.Description != "" {
		sb.WriteString(fmt.Sprintf("description: %s\n", skill.Description))
	}
	if skill.Keywords != "" {
		sb.WriteString(fmt.Sprintf("keywords: %s\n", skill.Keywords))
	}
	sb.WriteString(fmt.Sprintf("level: %d\n", skill.Level))
	sb.WriteString(fmt.Sprintf("status: %d\n", skill.Status))
	sb.WriteString("---\n\n")
	sb.WriteString(skill.Content)

	filePath := filepath.Join(dir, skill.Name+".md")
	if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
		slog.Error("写入技能文件失败", "name", skill.Name, "error", err)
		return
	}

	slog.Info("技能文件同步完成", "name", skill.Name, "path", filePath)
}

// DeleteSkill 删除技能文件
func (s *FileSyncService) DeleteSkill(name string) {
	contextRoot := config.Get().Agent.ContextRoot
	filePath := filepath.Join(contextRoot, "skills", name+".md")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		slog.Error("删除技能文件失败", "name", name, "error", err)
		return
	}
	slog.Info("技能文件已删除", "name", name)
}

// ---- 批量同步 ----

// SyncAll 全量同步所有数据到文件系统
func (s *FileSyncService) SyncAll() error {
	slog.Info("开始全量文件同步...")

	// 同步所有智能体元数据
	var agents []model.Agent
	s.DB.Find(&agents)
	for i := range agents {
		s.SyncAgent(&agents[i])
	}

	// 同步所有技能
	var skills []model.Skill
	s.DB.Find(&skills)
	for i := range skills {
		s.SyncSkill(&skills[i])
	}

	slog.Info("全量文件同步完成", "agents", len(agents), "skills", len(skills))
	return nil
}

// ---- 工具函数 ----

// writeYAML 将结构体写入 YAML 文件
func writeYAML(path string, v interface{}) {
	data, err := yaml.Marshal(v)
	if err != nil {
		slog.Error("YAML 序列化失败", "path", path, "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Error("写入 YAML 文件失败", "path", path, "error", err)
	}
}

// syncContentFile 同步内容文件（指针语义）
// nil=不操作，""=删除文件，非空=写入文件
func syncContentFile(path string, content *string) {
	if content == nil {
		return
	}
	if *content == "" {
		// 内容为空：删除文件（忽略不存在错误）
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			slog.Error("删除内容文件失败", "path", path, "error", err)
		}
		return
	}
	if err := os.WriteFile(path, []byte(*content), 0644); err != nil {
		slog.Error("写入文件失败", "path", path, "error", err)
	}
}

// parseJSONStringArray 解析 JSON 数组字符串为 []string
func parseJSONStringArray(jsonStr string) []string {
	if jsonStr == "" {
		return nil
	}
	var result []string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}
	return result
}
