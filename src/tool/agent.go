package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// WriteAgentOpts write_agent 工具的参数结构
type WriteAgentOpts struct {
	Title           string
	Description     string
	Keywords        string
	RolePrompt      string
	ModelID         string
	DocRoles        string
	AutoLoadFiles   string
	AutoLoadTools   string
	RuleContent     string
	SkillContent    string
	WorkflowContent string
	MemoryContent   string
}

// AgentWriter 写入 Agent 的接口（按 name 自动判断创建/更新，由 service.AgentService 实现）
type AgentWriter interface {
	WriteAgentFromTool(name string, opts WriteAgentOpts) (uint, bool, error)
}

// AgentLister 列出 Agent 的接口
type AgentLister interface {
	ListAgentsForTool() ([]map[string]interface{}, error)
}

// AgentLookup 查找智能体的接口（由 service.AgentService 实现）
type AgentLookup interface {
	FindAgentByName(name string) (uint, string, error)
}

// SubCallHandler 创建子会话并同步执行的接口（由 service.ChatRunner 实现）
type SubCallHandler interface {
	InvokeSubAgent(ctx context.Context, workspaceUUID string, workspaceID uint, agentID uint, agentName string, parentConvID uint, workDir string, task string) (string, error)
}

// WriteAgentTool AI 通过 tool_call 创建或更新 Agent（按 name 自动判断）
type WriteAgentTool struct {
	AgentWriter AgentWriter
}

func (t *WriteAgentTool) Name() string { return "write_agent" }
func (t *WriteAgentTool) Description() string {
	return "创建或更新 AI 智能体。按 name 自动判断：不存在则创建，已存在则更新。"
}
func (t *WriteAgentTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "智能体唯一英文名称（小写+下划线，如 code_reviewer）"},
			"title": {"type": "string", "description": "显示名称（创建时必填）"},
			"description": {"type": "string", "description": "功能描述"},
			"keywords": {"type": "string", "description": "关键词，逗号分隔"},
			"role_prompt": {"type": "string", "description": "角色设定内容（role.md）"},
			"model_id": {"type": "string", "description": "关联的模型 ID（如 claude-opus-4.6），空字符串表示不绑定"},
			"doc_roles": {"type": "string", "description": "项目文档角色标签，逗号分隔（如 backend,frontend,review）"},
			"auto_load_files": {"type": "string", "description": "自动加载文件类型 JSON 数组（如 [\"role\",\"rule\",\"workflow\",\"skill\",\"memory\"]）"},
			"auto_load_tools": {"type": "string", "description": "自动加载工具 JSON 数组（如 [\"read_files\",\"write_files\"]）"},
			"rule_content": {"type": "string", "description": "角色规则内容（rule.md）"},
			"skill_content": {"type": "string", "description": "角色技能速查内容（skill.md）"},
			"workflow_content": {"type": "string", "description": "工作流程内容（workflow.md）"},
			"memory_content": {"type": "string", "description": "角色记忆内容（memory.md）"}
		},
		"required": ["name"]
	}`)
}

func (t *WriteAgentTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Name            string `json:"name"`
		Title           string `json:"title"`
		Description     string `json:"description"`
		Keywords        string `json:"keywords"`
		RolePrompt      string `json:"role_prompt"`
		ModelID         string `json:"model_id"`
		DocRoles        string `json:"doc_roles"`
		AutoLoadFiles   string `json:"auto_load_files"`
		AutoLoadTools   string `json:"auto_load_tools"`
		RuleContent     string `json:"rule_content"`
		SkillContent    string `json:"skill_content"`
		WorkflowContent string `json:"workflow_content"`
		MemoryContent   string `json:"memory_content"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if params.Name == "" {
		return ErrorResult("name 为必填参数")
	}
	if t.AgentWriter == nil {
		return ErrorResult("系统错误: AgentWriter 未初始化")
	}

	opts := WriteAgentOpts{
		Title:           params.Title,
		Description:     params.Description,
		Keywords:        params.Keywords,
		RolePrompt:      params.RolePrompt,
		ModelID:         params.ModelID,
		DocRoles:        params.DocRoles,
		AutoLoadFiles:   params.AutoLoadFiles,
		AutoLoadTools:   params.AutoLoadTools,
		RuleContent:     params.RuleContent,
		SkillContent:    params.SkillContent,
		WorkflowContent: params.WorkflowContent,
		MemoryContent:   params.MemoryContent,
	}

	id, created, err := t.AgentWriter.WriteAgentFromTool(params.Name, opts)
	if err != nil {
		return ErrorResult("写入智能体失败: " + err.Error())
	}

	if created {
		return SuccessResult(fmt.Sprintf("智能体创建成功！ID=%d, 名称=%s", id, params.Name))
	}
	return SuccessResult(fmt.Sprintf("智能体(ID=%d, 名称=%s)更新成功！", id, params.Name))
}

// ListAgentsTool AI 通过 tool_call 列出所有 Agent
type ListAgentsTool struct {
	AgentLister AgentLister
}

func (t *ListAgentsTool) Name() string { return "list_agents" }
func (t *ListAgentsTool) Description() string {
	return "列出系统中所有可用的 AI 角色，包括 ID、名称、标题、描述等信息。"
}
func (t *ListAgentsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type": "object", "properties": {}}`)
}

func (t *ListAgentsTool) Execute(ctx context.Context, args string) *Result {
	if t.AgentLister == nil {
		return &Result{Content: "系统错误: AgentLister 未初始化"}
	}

	agents, err := t.AgentLister.ListAgentsForTool()
	if err != nil {
		return &Result{Content: "获取角色列表失败: " + err.Error()}
	}

	data, _ := json.MarshalIndent(agents, "", "  ")
	return &Result{Content: string(data)}
}

// CallAgentTool 调用其他智能体的工具（支持嵌套调用）
type CallAgentTool struct {
	AgentLookup    AgentLookup
	SubCallHandler SubCallHandler
}

func (t *CallAgentTool) Name() string { return "call_agent" }
func (t *CallAgentTool) Description() string {
	return "调用另一个 AI 智能体执行指定任务，并同步等待其完成，返回结果。支持嵌套调用（智能体可调用其他智能体）。"
}
func (t *CallAgentTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"agent_name": {
				"type": "string",
				"description": "目标智能体的唯一名称（如 olivia、ethan）"
			},
			"task": {
				"type": "string",
				"description": "分配给目标智能体的任务描述"
			}
		},
		"required": ["agent_name", "task"]
	}`)
}

func (t *CallAgentTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		AgentName string `json:"agent_name"`
		Task      string `json:"task"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if params.AgentName == "" {
		return ErrorResult("agent_name 不能为空")
	}
	if params.Task == "" {
		return ErrorResult("task 不能为空")
	}

	if t.AgentLookup == nil || t.SubCallHandler == nil {
		return ErrorResult("系统错误: CallAgentTool 未正确初始化")
	}

	wc := GetWorkContext(ctx)
	if wc == nil {
		return ErrorResult("工作上下文未初始化，无法调用子智能体")
	}
	if wc.WorkspaceUUID == "" {
		return ErrorResult("工作区 UUID 为空，无法调用子智能体")
	}

	agentID, agentName, err := t.AgentLookup.FindAgentByName(params.AgentName)
	if err != nil {
		return ErrorResult(fmt.Sprintf("智能体 '%s' 不存在: %v", params.AgentName, err))
	}

	subWorkDir := wc.WorkspaceDir
	if subWorkDir == "" {
		subWorkDir = wc.WorkDir
	}
	result, err := t.SubCallHandler.InvokeSubAgent(
		ctx,
		wc.WorkspaceUUID,
		0,
		agentID,
		agentName,
		wc.ConversationID,
		subWorkDir,
		params.Task,
	)
	if err != nil {
		return ErrorResult(fmt.Sprintf("调用智能体 '%s' 失败: %v", params.AgentName, err))
	}

	if strings.TrimSpace(result) == "" {
		return SuccessResult(fmt.Sprintf("[%s 的回复]\n（注意：该智能体未返回任何内容，可能未实际完成任务或未产出可验证结果。请检查相关文档、任务列表或子会话记录。）", params.AgentName))
	}

	return SuccessResult(fmt.Sprintf("[%s 的回复]\n%s", params.AgentName, result))
}
