package agentctx

// ContextLevel 上下文层级
type ContextLevel int

const (
	LevelSystem     ContextLevel = iota // 系统级（全局规则）
	LevelRole                           // 智能体级（role.md、rule.md、skill.md、workflow.md）
	LevelProject                        // 项目级（项目上下文）
	LevelProjectDoc                     // 项目级专项文档（docs/*.md，按角色标签过滤）
	LevelWorkDoc                        // 工作文档级（需求/分析/设计/任务等阶段产出文档）
	LevelMemory                         // 记忆级（memory.md）
)

// ContextBlock 上下文片段
type ContextBlock struct {
	Level   ContextLevel `json:"level"`
	Label   string       `json:"label"`   // 显示标签（如"角色设定"、"项目规则"）
	Content string       `json:"content"` // 实际内容
	Source  string       `json:"source"`  // 来源文件路径
}

// ContextParams 上下文构建参数
type ContextParams struct {
	ContextRoot   string   // 上下文根目录（如 ./context）
	ProjectPath   string   // 实际项目代码路径（注入到提示词，供 LLM 操作文件；同时从此路径加载项目上下文文件）
	WorkDir       string   // 工作目录路径（注入到提示词，供 LLM 创建工作文档）
	AgentName     string   // 智能体名称
	AutoLoadFiles []string // 自动加载文件类型（role/rule/workflow/skill/memory），为空不加载任何角色文件
	AgentDocRoles []string // 文件名前缀列表（如 backend/database/security/api/frontend），匹配 docs/ 下以该前缀开头的文档
	WorkDocsDir   string   // specs/ 目录路径（非空时加载工作区规格文档）
	SkillDocs     []ContextBlock
	TaskSummary   string // 当前阶段任务摘要（由 TaskService 生成，chatrunner 注入到对话消息末尾）
}

// BuildResult 构建结果，同时提供拼接文本和结构化块
type BuildResult struct {
	Text   string         // 拼接后的完整文本（OpenAI 等使用）
	Blocks []ContextBlock // 原始块列表（Claude 等使用，支持分块缓存）
}
