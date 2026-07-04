package agentctx

import (
	"fmt"
	"strings"
)

// Builder 上下文构建器，组装分级上下文为完整 system prompt
type Builder struct {
	Loader *Loader
}

// NewBuilder 创建上下文构建器
func NewBuilder() *Builder {
	return &Builder{Loader: NewLoader()}
}

// Build 根据参数构建完整的 system prompt
// 统一加载顺序：System（system.md + global.md + user.md） → Project → Specs文档 → Role（含 rule/workflow/skill/memory） → 关联技能 → TaskSummary
func (b *Builder) Build(params ContextParams) BuildResult {
	var blocks []ContextBlock

	// 1. 系统级上下文
	blocks = append(blocks, b.Loader.LoadSystem(params.ContextRoot)...)

	// 2. 项目级上下文（从实际项目路径加载 AGENTS.md/CONTEXT.md/README.md）
	blocks = append(blocks, b.Loader.LoadProject(params.ProjectPath)...)

	// 2.5 项目级专项文档（docs/*.md，按 AgentDocRoles 过滤）
	if params.ProjectPath != "" && len(params.AgentDocRoles) > 0 {
		blocks = append(blocks, b.Loader.LoadProjectDocs(params.ProjectPath, params.AgentDocRoles)...)
	}

	// 3. 工作文档（specs/ 目录）
	if params.WorkDocsDir != "" {
		blocks = append(blocks, b.Loader.LoadWorkDocs(params.WorkDocsDir)...)
	}

	// 4. 智能体级上下文（role/rule/workflow/skill/memory，统一由 AutoLoadFiles 过滤）
	roleBlocks := b.Loader.LoadRole(params.ContextRoot, params.AgentName)
	if len(params.AutoLoadFiles) > 0 {
		roleBlocks = filterRoleBlocks(roleBlocks, params.AutoLoadFiles)
	} else {
		roleBlocks = nil
	}
	blocks = append(blocks, roleBlocks...)

	// 5. agent.yaml skills 关联技能（由运行时按需注入）
	blocks = append(blocks, params.SkillDocs...)

	// 6. 任务列表（放最后，前面的稳定内容利于 API 前缀缓存命中）
	if params.TaskSummary != "" {
		blocks = append(blocks, ContextBlock{
			Level:   LevelProject,
			Label:   "当前任务",
			Content: params.TaskSummary,
			Source:  "task_summary",
		})
	}

	if len(blocks) == 0 {
		return BuildResult{}
	}
	return BuildResult{Text: FormatBlocks(blocks), Blocks: blocks}
}

// shouldLoadFile 检查是否应加载指定类型的文件
// autoLoadFiles 为空时不加载任何文件，非空时只加载列表中包含的类型
func shouldLoadFile(autoLoadFiles []string, fileType string) bool {
	if len(autoLoadFiles) == 0 {
		return false
	}
	for _, f := range autoLoadFiles {
		if f == fileType {
			return true
		}
	}
	return false
}

// fileTypeMap 角色上下文文件名到类型的映射
var fileTypeMap = map[string]string{
	"role.md":     "role",
	"rule.md":     "rule",
	"skill.md":    "skill",
	"workflow.md": "workflow",
	"memory.md":   "memory",
}

// filterRoleBlocks 根据 AutoLoadFiles 配置过滤角色级上下文块
func filterRoleBlocks(blocks []ContextBlock, autoLoadFiles []string) []ContextBlock {
	allowed := make(map[string]bool)
	for _, f := range autoLoadFiles {
		allowed[f] = true
	}

	var filtered []ContextBlock
	for _, block := range blocks {
		// 从 Source 路径中提取文件名判断类型
		parts := strings.Split(block.Source, "/")
		if len(parts) > 0 {
			fileName := parts[len(parts)-1]
			if fileType, ok := fileTypeMap[fileName]; ok {
				if allowed[fileType] {
					filtered = append(filtered, block)
				}
				continue
			}
		}
		// 无法识别的块保留
		filtered = append(filtered, block)
	}
	return filtered
}

// FormatBlocks 将上下文块格式化为结构化文本
func FormatBlocks(blocks []ContextBlock) string {
	var sb strings.Builder

	for i, block := range blocks {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("<%s>\n%s\n</%s>",
			block.Label, block.Content, block.Label))
	}

	return sb.String()
}

