package tool

import (
	"context"
	"encoding/json"

	"agent_flow/src/provider"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// Tool 工具接口，LLM 可调用的工具需实现此接口
type Tool interface {
	// Name 工具唯一名称
	Name() string
	// Description 工具描述（给 LLM 看）
	Description() string
	// Parameters JSON Schema 定义参数格式
	Parameters() json.RawMessage
	// Execute 执行工具，args 为 JSON 字符串
	Execute(ctx context.Context, args string) *Result
}

// Result 工具执行结果
type Result struct {
	Content string      `json:"content"`            // 返回内容
	IsError bool        `json:"is_error,omitempty"` // 是否为错误
	Images  []ImageData `json:"images,omitempty"`   // 图片附件（由 ChatRunner 统一处理视觉逻辑）
}

// ImageData 工具产出的图片数据
type ImageData struct {
	Path      string `json:"path"`       // 文件路径（相对于工作区）
	MediaType string `json:"media_type"` // MIME 类型（image/png, image/jpeg 等）
	Data      string `json:"data"`       // base64 编码数据（可选，为空时从 Path 读取）
}

// ErrorResult 创建错误结果
func ErrorResult(msg string) *Result {
	return &Result{Content: msg, IsError: true}
}

// SuccessResult 创建成功结果
func SuccessResult(content string) *Result {
	return &Result{Content: content}
}

// ToToolDef 将 Tool 接口转为 LLM ToolDef
func ToToolDef(t Tool) provider.ToolDef {
	return provider.ToolDef{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters:  t.Parameters(),
	}
}

// ToToolDefs 批量转换
func ToToolDefs(tools []Tool) []provider.ToolDef {
	defs := make([]provider.ToolDef, len(tools))
	for i, t := range tools {
		defs[i] = ToToolDef(t)
	}
	return defs
}

// WorkContext 工具执行上下文（限制工作目录等）
type WorkContext struct {
	WorkspaceID    uint     // 工作区 DB 主键（减少工具内 UUID→ID 查询）
	WorkDir        string   // 工具工作目录（代码操作根目录，ProjectPath 存在时指向项目目录）
	WorkspaceDir   string   // 工作区目录（始终为 working/{uuid}/，不被 ProjectPath 覆盖）
	ProjectPath    string   // 项目代码路径（供子会话继承，确保子智能体能访问项目文件）
	AllowedDirs    []string // 额外允许访问的目录
	WorkspaceUUID  string   // 工作区 UUID（供 call_agent 工具创建子会话）
	ConversationID uint     // 当前会话 ID（子会话 ParentID 指向此）
}

// AgentInfo 当前智能体信息（与工作区无关，属于智能体自身属性）
type AgentInfo struct {
	Name        string // 智能体名称
	ContextRoot string // 上下文根目录（如 ./context）
}

// WorkingDBGetter 提供按工作区 UUID 获取 DB 连接的能力
// 由 service.WorkingDBManager 实现，通过接口注入避免循环依赖
type WorkingDBGetter interface {
	GetDB(workspaceUUID string) (*gorm.DB, error)
}

// TaskServicer 任务服务接口（避免 tool→service 循环依赖）
// 由 service.TaskService 实现
type TaskServicer interface {
	Create(workspaceUUID string, req *model.TaskCreateReq) (*model.Task, error)
	GetByID(workspaceUUID string, id uint) (*model.Task, error)
	ListWithSubTasks(workspaceUUID string) ([]model.Task, error)
	ListSubTasks(workspaceUUID string, parentID uint) ([]model.Task, error)
	Update(workspaceUUID string, id uint, req *model.TaskUpdateReq) (*model.Task, error)
	DeleteWithValidation(workspaceUUID string, id uint) (string, int64, error)
	Subdivide(workspaceUUID string, parentID uint, reqs []*model.TaskCreateReq) ([]*model.Task, error)
}
type ctxKey string

const workContextKey ctxKey = "work_context"
const agentInfoKey ctxKey = "agent_info"

// WithWorkContext 将 WorkContext 注入到 context 中
func WithWorkContext(ctx context.Context, wc *WorkContext) context.Context {
	return context.WithValue(ctx, workContextKey, wc)
}

// GetWorkContext 从 context 中获取 WorkContext
func GetWorkContext(ctx context.Context) *WorkContext {
	if wc, ok := ctx.Value(workContextKey).(*WorkContext); ok {
		return wc
	}
	return nil
}

// WithAgentInfo 将 AgentInfo 注入到 context 中
func WithAgentInfo(ctx context.Context, ai *AgentInfo) context.Context {
	return context.WithValue(ctx, agentInfoKey, ai)
}

// GetAgentInfo 从 context 中获取 AgentInfo
func GetAgentInfo(ctx context.Context) *AgentInfo {
	if ai, ok := ctx.Value(agentInfoKey).(*AgentInfo); ok {
		return ai
	}
	return nil
}
