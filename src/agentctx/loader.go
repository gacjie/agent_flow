package agentctx

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Loader 上下文文件加载器
type Loader struct{}

// NewLoader 创建加载器
func NewLoader() *Loader {
	return &Loader{}
}

// LoadRole 加载智能体级上下文（从 context/agents/{name}/ 加载）
// 加载顺序：role.md → rule.md → workflow.md → skill.md → memory.md
// 全部受 AutoLoadFiles 过滤（由 builder.filterRoleBlocks 处理）
func (l *Loader) LoadRole(contextRoot, agentName string) []ContextBlock {
	agentDir := filepath.Join(contextRoot, "agents", agentName)
	if !dirExists(agentDir) {
		return nil
	}

	var blocks []ContextBlock

	// role.md — 角色设定（核心 system prompt）
	if content := readFileIfExists(filepath.Join(agentDir, "role.md")); content != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelRole,
			Label:   "角色设定",
			Content: content,
			Source:  filepath.Join("agents", agentName, "role.md"),
		})
	}

	// rule.md — 角色规则
	if content := readFileIfExists(filepath.Join(agentDir, "rule.md")); content != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelRole,
			Label:   "角色规则",
			Content: content,
			Source:  filepath.Join("agents", agentName, "rule.md"),
		})
	}

	// workflow.md — 角色工作流
	if content := readFileIfExists(filepath.Join(agentDir, "workflow.md")); content != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelRole,
			Label:   "工作流程",
			Content: content,
			Source:  filepath.Join("agents", agentName, "workflow.md"),
		})
	}

	// skill.md — 角色技能描述
	if content := readFileIfExists(filepath.Join(agentDir, "skill.md")); content != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelRole,
			Label:   "技能说明",
			Content: content,
			Source:  filepath.Join("agents", agentName, "skill.md"),
		})
	}

	// memory.md — 智能体专属记忆（跨工作区持久生效）
	if content := readFileIfExists(filepath.Join(agentDir, "memory.md")); content != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelMemory,
			Label:   "角色记忆",
			Content: content,
			Source:  filepath.Join("agents", agentName, "memory.md"),
		})
	}

	return blocks
}

// LoadProject 加载项目级上下文（从实际项目路径加载 AGENTS.md/CONTEXT.md/README.md）
func (l *Loader) LoadProject(projectPath string) []ContextBlock {
	if projectPath == "" {
		return nil
	}

	if !dirExists(projectPath) {
		return nil
	}

	var blocks []ContextBlock

	contextFiles := []struct {
		name  string
		label string
	}{
		{"AGENTS.md", "项目概述"},
		{"CONTEXT.md", "项目上下文"},
		{"README.md", "项目说明"},
	}

	for _, cf := range contextFiles {
		path := filepath.Join(projectPath, cf.name)
		content := readFileIfExists(path)
		if content != "" {
			blocks = append(blocks, ContextBlock{
				Level:   LevelProject,
				Label:   cf.label,
				Content: content,
				Source:  cf.name,
			})
			break
		}
	}

	return blocks
}

// LoadSystem 加载系统级上下文（全局规则，如有）
func (l *Loader) LoadSystem(contextRoot string) []ContextBlock {
	path := filepath.Join(contextRoot, "system.md")
	content := readFileIfExists(path)
	if content == "" {
		return nil
	}
	return []ContextBlock{{
		Level:   LevelSystem,
		Label:   "系统规则",
		Content: content,
		Source:  "system.md",
	}}
}

// docLabelMap 文档名 → 中文标签
var docLabelMap = map[string]string{
	"requirement": "需求文档",
	"design":      "系统设计",
	"backend":     "后端开发记录",
	"css":         "CSS开发记录",
	"javascript":  "JavaScript开发记录",
	"test-report": "测试报告",
	"docs":        "文档记录",
	"review":      "代码审查报告",
}

// docLabelFromFilename 从文件名生成中文标签
// 例如：requirement.md → "需求文档"，backend-fix.md → "backend-fix"
func docLabelFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	if label, ok := docLabelMap[name]; ok {
		return label
	}
	return name
}

// LoadWorkDocs 扫描 specs/ 目录中所有 .md 文件，按文件修改时间排序（早期修改在前）
// 返回单一列表，所有 specs 文档统一作为稳定文档加载到系统上下文
func (l *Loader) LoadWorkDocs(specsDir string) []ContextBlock {
	if !dirExists(specsDir) {
		return nil
	}

	entries, err := os.ReadDir(specsDir)
	if err != nil {
		slog.Debug("读取 specs 目录失败", "path", specsDir, "error", err)
		return nil
	}

	// 收集文件及修改时间
	type fileEntry struct {
		name    string
		modTime int64
	}
	var files []fileEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{name: name, modTime: info.ModTime().UnixNano()})
	}

	// 按修改时间排序（早期修改在前，最近修改在后）
	for i := 0; i < len(files); i++ {
		for j := i + 1; j < len(files); j++ {
			if files[j].modTime < files[i].modTime {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	var blocks []ContextBlock
	for _, f := range files {
		content := readFileIfExists(filepath.Join(specsDir, f.name))
		if content == "" {
			continue
		}
		blocks = append(blocks, ContextBlock{
			Level:   LevelWorkDoc,
			Label:   docLabelFromFilename(f.name),
			Content: content,
			Source:  "specs/" + f.name,
		})
	}

	return blocks
}

// ---- 工具函数 ----

func readFileIfExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Debug("读取上下文文件失败", "path", path, "error", err)
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
