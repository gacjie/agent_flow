package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"agent_flow/src/model"
)

// MCPServerStatus MCP 服务器运行状态
type MCPServerStatus struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	Error     string   `json:"error,omitempty"`
	ToolCount int      `json:"tool_count"`
	ToolNames []string `json:"tool_names,omitempty"`
}

type mcpServerEntry struct {
	mcpToolID uint
	name      string
	client    *MCPClient
	tools     []MCPToolDef
	state     string
	err       string
}

// MCPManager MCP 服务器进程管理器
type MCPManager struct {
	mu       sync.RWMutex
	servers  map[string]*mcpServerEntry
	registry *Registry
}

// NewMCPManager 创建 MCP 进程管理器
func NewMCPManager(registry *Registry) *MCPManager {
	return &MCPManager{
		servers:  make(map[string]*mcpServerEntry),
		registry: registry,
	}
}

// StartServer 启动 MCP 服务器：创建进程 → 握手 → 发现工具 → 注册到 Registry
func (m *MCPManager) StartServer(mcpTool *model.MCPTool) error {
	m.mu.Lock()
	if entry, ok := m.servers[mcpTool.Name]; ok {
		if (entry.state == "running" && entry.client != nil && entry.client.Alive()) || entry.state == "starting" {
			m.mu.Unlock()
			return nil
		}
	}
	m.servers[mcpTool.Name] = &mcpServerEntry{
		mcpToolID: mcpTool.ID,
		name:      mcpTool.Name,
		state:     "starting",
	}
	m.mu.Unlock()

	parts := parseCommandLine(mcpTool.Command)
	if len(parts) == 0 {
		m.setServerError(mcpTool.Name, mcpTool.ID, "MCP 服务器命令为空")
		return fmt.Errorf("MCP 服务器命令为空")
	}

	var extraArgs []string
	if mcpTool.Args != "" {
		if err := json.Unmarshal([]byte(mcpTool.Args), &extraArgs); err != nil {
			m.setServerError(mcpTool.Name, mcpTool.ID, "参数 JSON 格式错误: "+err.Error())
			return fmt.Errorf("参数 JSON 格式错误: %w", err)
		}
	}
	allArgs := append(parts[1:], extraArgs...)

	var envSlice []string
	if mcpTool.Env != "" {
		var envMap map[string]string
		if err := json.Unmarshal([]byte(mcpTool.Env), &envMap); err != nil {
			m.setServerError(mcpTool.Name, mcpTool.ID, "环境变量 JSON 格式错误: "+err.Error())
			return fmt.Errorf("环境变量 JSON 格式错误: %w", err)
		}
		for k, v := range envMap {
			envSlice = append(envSlice, k+"="+v)
		}
	}

	client := NewMCPClient(parts[0], allArgs, envSlice)
	client.OnToolsChanged(func() {
		m.refreshTools(mcpTool.Name)
	})

	timeout := 60 * time.Second
	if mcpTool.Timeout > 0 {
		timeout = time.Duration(mcpTool.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	slog.Info("正在启动 MCP 服务器", "name", mcpTool.Name, "command", mcpTool.Command)

	if err := client.Start(ctx); err != nil {
		m.setServerError(mcpTool.Name, mcpTool.ID, err.Error())
		return err
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		client.Close()
		m.setServerError(mcpTool.Name, mcpTool.ID, "获取工具列表失败: "+err.Error())
		return fmt.Errorf("获取工具列表失败: %w", err)
	}

	m.mu.Lock()
	entry := &mcpServerEntry{
		mcpToolID: mcpTool.ID,
		name:      mcpTool.Name,
		client:    client,
		tools:     tools,
		state:     "running",
	}
	m.servers[mcpTool.Name] = entry

	for _, td := range tools {
		regName := "mcp__" + mcpTool.Name + "__" + td.Name
		adapter := &MCPToolAdapter{
			name:        regName,
			description: td.Description,
			params:      td.InputSchema,
			serverName:  mcpTool.Name,
			toolName:    td.Name,
			manager:     m,
		}
		m.registry.Register(adapter)
	}
	m.mu.Unlock()

	slog.Info("MCP 服务器启动成功", "name", mcpTool.Name, "tools", len(tools))

	go m.monitorProcess(mcpTool.Name, client)

	return nil
}

// StopServer 停止 MCP 服务器并注销所有工具
func (m *MCPManager) StopServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.servers[name]
	if !ok {
		return nil
	}

	m.unregisterTools(entry)

	if entry.client != nil {
		entry.client.Close()
	}

	entry.state = "stopped"
	entry.err = ""
	entry.client = nil
	entry.tools = nil

	slog.Info("MCP 服务器已停止", "name", name)
	return nil
}

// RestartServer 重启 MCP 服务器
func (m *MCPManager) RestartServer(mcpTool *model.MCPTool) error {
	m.mu.Lock()
	if entry, ok := m.servers[mcpTool.Name]; ok {
		if entry.state == "starting" {
			m.mu.Unlock()
			return fmt.Errorf("MCP 服务器正在启动中，请稍后重试")
		}
		m.unregisterTools(entry)
		if entry.client != nil {
			entry.client.Close()
		}
	}
	delete(m.servers, mcpTool.Name)
	m.mu.Unlock()

	return m.StartServer(mcpTool)
}

// GetStatus 获取单个服务器状态
func (m *MCPManager) GetStatus(name string) MCPServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.servers[name]
	if !ok {
		return MCPServerStatus{Name: name, State: "stopped"}
	}

	status := MCPServerStatus{
		Name:      entry.name,
		State:     entry.state,
		Error:     entry.err,
		ToolCount: len(entry.tools),
	}

	for _, t := range entry.tools {
		status.ToolNames = append(status.ToolNames, t.Name)
	}

	return status
}

// GetAllStatuses 获取所有已配置服务器的状态
func (m *MCPManager) GetAllStatuses() map[string]MCPServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]MCPServerStatus, len(m.servers))
	for name, entry := range m.servers {
		status := MCPServerStatus{
			Name:      entry.name,
			State:     entry.state,
			Error:     entry.err,
			ToolCount: len(entry.tools),
		}
		for _, t := range entry.tools {
			status.ToolNames = append(status.ToolNames, t.Name)
		}
		result[name] = status
	}
	return result
}

// GetClient 获取指定服务器的 MCPClient（供 MCPToolAdapter 使用）
func (m *MCPManager) GetClient(serverName string) (*MCPClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.servers[serverName]
	if !ok || entry.state != "running" || entry.client == nil {
		return nil, false
	}
	return entry.client, true
}

// ShutdownAll 关闭所有 MCP 服务器（应用退出时调用）
func (m *MCPManager) ShutdownAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, entry := range m.servers {
		if entry.client != nil {
			slog.Info("正在关闭 MCP 服务器", "name", name)
			entry.client.Close()
		}
	}
	m.servers = make(map[string]*mcpServerEntry)
}

// unregisterTools 从 Registry 注销服务器的所有工具（调用方需持有锁）
func (m *MCPManager) unregisterTools(entry *mcpServerEntry) {
	for _, td := range entry.tools {
		regName := "mcp__" + entry.name + "__" + td.Name
		m.registry.Unregister(regName)
	}
}

// monitorProcess 监控 MCP 服务器进程，崩溃时更新状态
func (m *MCPManager) monitorProcess(name string, client *MCPClient) {
	<-client.done

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.servers[name]
	if !ok || entry.client != client {
		return
	}

	m.unregisterTools(entry)

	entry.state = "error"
	entry.err = "进程异常退出"
	if client.err != nil {
		entry.err = client.err.Error()
	}
	entry.client = nil

	slog.Warn("MCP 服务器进程异常退出", "name", name, "error", entry.err)
}

// setServerError 设置服务器错误状态（用于 StartServer 错误路径）
func (m *MCPManager) setServerError(name string, mcpToolID uint, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[name] = &mcpServerEntry{
		mcpToolID: mcpToolID,
		name:      name,
		state:     "error",
		err:       errMsg,
	}
}

// refreshTools 重新获取工具列表并更新 Registry（收到 tools/list_changed 通知时调用）
func (m *MCPManager) refreshTools(name string) {
	m.mu.RLock()
	entry, ok := m.servers[name]
	if !ok || entry.state != "running" || entry.client == nil {
		m.mu.RUnlock()
		return
	}
	client := entry.client
	m.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tools, err := client.ListTools(ctx)
	if err != nil {
		slog.Warn("刷新 MCP 工具列表失败", "name", name, "error", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok = m.servers[name]
	if !ok || entry.client != client {
		return
	}

	m.unregisterTools(entry)
	entry.tools = tools
	for _, td := range tools {
		regName := "mcp__" + name + "__" + td.Name
		adapter := &MCPToolAdapter{
			name:        regName,
			description: td.Description,
			params:      td.InputSchema,
			serverName:  name,
			toolName:    td.Name,
			manager:     m,
		}
		m.registry.Register(adapter)
	}

	slog.Info("MCP 工具列表已刷新", "name", name, "tools", len(tools))
}
