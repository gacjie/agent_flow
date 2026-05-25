package service

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"agent_flow/src/model"

	"go.yaml.in/yaml/v3"
	"gorm.io/gorm"
)

// SkillSeedConfig 技能文件 YAML front matter 结构
type SkillSeedConfig struct {
	Name        string `yaml:"name"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
	Keywords    string `yaml:"keywords"`
	Level       int    `yaml:"level"`
	Status      int    `yaml:"status"`
	Sort        int    `yaml:"sort"`
}

// SkillSeedService 从文件系统扫描本地技能并 upsert 到数据库
type SkillSeedService struct {
	db *gorm.DB
}

// NewSkillSeedService 创建 SkillSeedService
func NewSkillSeedService(db *gorm.DB) *SkillSeedService {
	return &SkillSeedService{db: db}
}

// Seed 扫描 {contextRoot}/skills/ 目录，将所有本地技能 upsert 到数据库
func (s *SkillSeedService) Seed(contextRoot string) error {
	skillsDir := filepath.Join(contextRoot, "skills")

	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		slog.Debug("内置技能目录不存在，跳过加载", "dir", skillsDir)
		return nil
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return fmt.Errorf("读取技能目录失败: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(skillsDir, entry.Name())

		cfg, content, err := s.parseSkillFile(filePath)
		if err != nil {
			slog.Warn("解析技能文件失败", "file", filePath, "error", err)
			continue
		}

		if err := s.upsert(cfg, content); err != nil {
			slog.Warn("内置技能 upsert 失败", "name", cfg.Name, "error", err)
			continue
		}

		count++
	}

	slog.Info("内置技能加载完成", "count", count)
	return nil
}

// parseSkillFile 解析技能 Markdown 文件，提取 YAML front matter 和正文内容
func (s *SkillSeedService) parseSkillFile(filePath string) (*SkillSeedConfig, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("读取技能文件失败: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var frontMatter strings.Builder
	var body strings.Builder
	inFrontMatter := false
	frontMatterDone := false
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()

		if firstLine {
			firstLine = false
			if strings.TrimSpace(line) == "---" {
				inFrontMatter = true
				continue
			}
			// 没有 front matter，整个文件作为正文
			body.WriteString(line)
			body.WriteString("\n")
			frontMatterDone = true
			continue
		}

		if inFrontMatter {
			if strings.TrimSpace(line) == "---" {
				inFrontMatter = false
				frontMatterDone = true
				continue
			}
			frontMatter.WriteString(line)
			frontMatter.WriteString("\n")
			continue
		}

		if frontMatterDone {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, "", fmt.Errorf("读取文件内容失败: %w", err)
	}

	// 解析 YAML front matter
	var cfg SkillSeedConfig
	if fm := frontMatter.String(); fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &cfg); err != nil {
			return nil, "", fmt.Errorf("解析 YAML front matter 失败: %w", err)
		}
	}

	// name 必须存在
	if cfg.Name == "" {
		// 从文件名推导 name
		base := filepath.Base(filePath)
		cfg.Name = strings.TrimSuffix(base, ".md")
	}

	// label 默认使用 name
	if cfg.Label == "" {
		cfg.Label = cfg.Name
	}

	content := strings.TrimSpace(body.String())

	return &cfg, content, nil
}

// upsert 将技能配置写入数据库（不存在则创建，已存在则按本地文件更新缓存）
func (s *SkillSeedService) upsert(cfg *SkillSeedConfig, content string) error {
	status := cfg.Status
	if status == 0 {
		status = 1
	}

	level := cfg.Level
	if level < 1 || level > 3 {
		level = 1
	}

	var existing model.Skill
	result := s.db.Where("name = ?", cfg.Name).First(&existing)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		skill := model.Skill{
			Name:        cfg.Name,
			Label:       cfg.Label,
			Description: cfg.Description,
			Keywords:    cfg.Keywords,
			Content:     content,
			Level:       level,
			Status:      status,
			Sort:        cfg.Sort,
		}
		if err := s.db.Create(&skill).Error; err != nil {
			return fmt.Errorf("创建技能失败: %w", err)
		}
		slog.Info("内置技能已创建", "name", cfg.Name, "label", cfg.Label)
	} else if result.Error == nil {
		updates := map[string]interface{}{
			"label":       cfg.Label,
			"description": cfg.Description,
			"keywords":    cfg.Keywords,
			"content":     content,
			"level":       level,
			"status":      status,
			"sort":        cfg.Sort,
		}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			return fmt.Errorf("更新技能失败: %w", err)
		}
		slog.Debug("内置技能已更新", "name", cfg.Name)
	} else {
		return fmt.Errorf("查询技能失败: %w", result.Error)
	}

	return nil
}
