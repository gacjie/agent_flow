package agentctx

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
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

	// workflow.md 不再加载到 system prompt，由 WorkflowEngine 按步骤解析并注入
	// （见 service/workflow_engine.go）

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

// LoadSystem 加载系统级上下文（system.md + global.md + user.md）
func (l *Loader) LoadSystem(contextRoot string) []ContextBlock {
	var blocks []ContextBlock

	// system.md — 系统环境信息（启动时自动生成）
	if content := readFileIfExists(filepath.Join(contextRoot, "system.md")); content != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelSystem,
			Label:   "系统规则",
			Content: content,
			Source:  "system.md",
		})
	}

	// global.md — 全局提示词（跟随项目）
	if content := readFileIfExists(filepath.Join(contextRoot, "global.md")); content != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelSystem,
			Label:   "全局提示词",
			Content: content,
			Source:  "global.md",
		})
	}

	// user.md — 用户自定义提示词（不存在则跳过）
	if content := readFileIfExists(filepath.Join(contextRoot, "user.md")); content != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelSystem,
			Label:   "用户提示词",
			Content: content,
			Source:  "user.md",
		})
	}

	return blocks
}

// LoadProjectDocs 扫描 {projectPath}/docs/*.md，按文件名前缀与 agentRoles 匹配加载
// 文件名前缀为第一个 "-" 前的部分（如 backend-standards.md 前缀为 backend），大小写不敏感
func (l *Loader) LoadProjectDocs(projectPath string, agentRoles []string) []ContextBlock {
	if len(agentRoles) == 0 {
		return nil
	}

	docsDir := filepath.Join(projectPath, "docs")
	if !dirExists(docsDir) {
		return nil
	}

	entries, err := os.ReadDir(docsDir)
	if err != nil {
		slog.Debug("读取 docs 目录失败", "path", docsDir, "error", err)
		return nil
	}

	roleSet := make(map[string]bool, len(agentRoles))
	for _, r := range agentRoles {
		roleSet[strings.TrimSpace(strings.ToLower(r))] = true
	}

	// 收集文件名并排序（稳定输出）
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	var blocks []ContextBlock
	for _, name := range names {
		// 提取前缀：第一个 "-" 前的部分（转小写）
		base := strings.TrimSuffix(strings.ToLower(name), ".md")
		prefix := base
		if idx := strings.Index(base, "-"); idx > 0 {
			prefix = base[:idx]
		}
		if !roleSet[prefix] {
			continue
		}

		content := readFileIfExists(filepath.Join(docsDir, name))
		if content == "" {
			continue
		}

		// 向后兼容：剥离已有文档的 front matter
		if strings.HasPrefix(strings.TrimSpace(content), "---") {
			_, body := parseFrontMatterRoles(content)
			content = strings.TrimSpace(body)
			if content == "" {
				continue
			}
		}

		label := strings.TrimSuffix(name, ".md")
		blocks = append(blocks, ContextBlock{
			Level:   LevelProjectDoc,
			Label:   label,
			Content: content,
			Source:  "docs/" + name,
		})
	}

	return blocks
}

// parseFrontMatterRoles 提取 YAML front matter 中的 roles 字段
// 支持内联数组 roles: [a, b] 和块列表 - a 两种格式
func parseFrontMatterRoles(content string) (roles []string, body string) {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(strings.TrimRight(lines[0], "\n")) != "---" {
		return nil, content
	}

	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(strings.TrimRight(lines[i], "\n")) == "---" {
			endIdx = i
			break
		}
	}
	if endIdx < 0 {
		return nil, content
	}

	// 解析 front matter 中的 roles
	inRolesBlock := false
	for i := 1; i < endIdx; i++ {
		line := strings.TrimRight(lines[i], "\n")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "roles:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "roles:"))
			if value != "" {
				// 内联数组：roles: [a, b, c]
				value = strings.Trim(value, "[]")
				for _, r := range strings.Split(value, ",") {
					r = strings.TrimSpace(r)
					if r != "" {
						roles = append(roles, r)
					}
				}
				inRolesBlock = false
			} else {
				inRolesBlock = true
			}
			continue
		}

		if inRolesBlock {
			if strings.HasPrefix(trimmed, "- ") {
				r := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
				if r != "" {
					roles = append(roles, r)
				}
			} else {
				inRolesBlock = false
			}
		}
	}

	// body 为 front matter 之后的内容
	var sb strings.Builder
	for i := endIdx + 1; i < len(lines); i++ {
		sb.WriteString(lines[i])
	}
	body = sb.String()

	return roles, body
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
