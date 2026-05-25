package tool

import (
	"strings"
	"sync"
)

// Registry 工具注册表
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry 创建工具注册表
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register 注册工具
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
}

// Get 按名称获取工具
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List 列出所有已注册的工具
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// Unregister 注销工具（按名称删除）
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// RegisterAll 批量注册工具
func (r *Registry) RegisterAll(tools []Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tools {
		r.tools[t.Name()] = t
	}
}

// GetByNames 按名称列表获取工具，names 为空或包含 "all" 时返回全部
func (r *Registry) GetByNames(names []string) []Tool {
	if len(names) == 0 {
		return r.List()
	}

	for _, n := range names {
		if n == "all" {
			return r.List()
		}
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Tool
	for _, name := range names {
		if t, ok := r.tools[name]; ok {
			result = append(result, t)
		} else {
			prefix := "mcp__" + name + "__"
			for regName, t := range r.tools {
				if strings.HasPrefix(regName, prefix) {
					result = append(result, t)
				}
			}
		}
	}
	return result
}

// Names 返回所有已注册工具的名称
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry 默认全局注册表（包含无依赖内置工具）
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	r.Register(&DeleteFileTool{})
	r.Register(&ListFilesTool{})
	r.Register(&SearchFilesTool{})
	r.Register(&RunCommandTool{})
	r.Register(&UpdateMemoryTool{})
	r.Register(&WebSearchTool{})
	r.Register(&WriteWorkDocTool{})
	r.Register(&ListWorkDocsTool{})
	r.Register(&ReadWorkDocTool{})
	r.Register(&DeleteWorkDocTool{})
	r.Register(&ReadContextTool{})
	r.Register(&WriteContextTool{})
	r.Register(&DeleteContextTool{})
	r.Register(&ContextListTool{})
	r.Register(&SearchContextTool{})
	r.Register(&BrowserActionTool{})
	r.Register(&AnalysisFileTool{})
	r.Register(&AnalysisWorkDocTool{})
	r.Register(&AnalysisContextTool{})
	r.Register(&DiagnoseFilesTool{})
	return r
}

// AgentServiceDeps 组合了 Agent 管理工具所需的全部依赖接口
// service.AgentService 同时实现了这三个接口
type AgentServiceDeps interface {
	AgentWriter
	AgentLister
	AgentLookup
}

// RegisterTaskTools 注册任务管理工具（需要 TaskServicer 依赖）
func RegisterTaskTools(r *Registry, taskSvc TaskServicer) {
	r.Register(&TaskListsTool{TaskService: taskSvc})
	r.Register(&TaskWritersTool{TaskService: taskSvc})
	r.Register(&TaskDeletesTool{TaskService: taskSvc})
}
// RegisterAgentTools 注册 3 个 Agent 管理工具（需要服务层依赖，在路由初始化后调用）
func RegisterAgentTools(r *Registry, svc AgentServiceDeps, handler SubCallHandler) {
	r.Register(&WriteAgentTool{AgentWriter: svc})
	r.Register(&ListAgentsTool{AgentLister: svc})
	r.Register(&CallAgentTool{AgentLookup: svc, SubCallHandler: handler})
}
