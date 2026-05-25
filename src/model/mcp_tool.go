package model

// MCPTool MCP 工具（外部工具，替代原 ExternalTool，支持完整命令行配置）
type MCPTool struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;size:100" json:"name"`
	Label       string `gorm:"size:100" json:"label"`
	Description string `gorm:"size:500" json:"description"`
	Command     string `gorm:"size:500" json:"command"`    // 完整命令行（如 /path/to/bin 或 npx -y @mcp/server-xxx）
	Args        string `gorm:"type:text" json:"args"`      // JSON 数组，额外参数
	Env         string `gorm:"type:text" json:"env"`       // JSON 对象，环境变量 {"KEY":"VALUE"}
	Parameters  string `gorm:"type:text" json:"parameters"` // JSON Schema 参数定义
	Category    string `gorm:"size:50;index" json:"category"`
	Version     string `gorm:"size:50" json:"version"`
	Timeout     int    `gorm:"default:30" json:"timeout"` // 超时秒数
	Status      int    `gorm:"default:1" json:"status"`   // 1=启用 0=禁用
	Sort        int    `gorm:"default:0" json:"sort"`
}

// MCPToolCreateReq 创建 MCP 工具请求
type MCPToolCreateReq struct {
	Name        string `json:"name" validate:"required,max=100"`
	Label       string `json:"label" validate:"required,max=100"`
	Description string `json:"description" validate:"max=500"`
	Command     string `json:"command" validate:"required,max=500"`
	Args        string `json:"args"`
	Env         string `json:"env"`
	Parameters  string `json:"parameters"`
	Category    string `json:"category" validate:"max=50"`
	Version     string `json:"version" validate:"max=50"`
	Timeout     int    `json:"timeout"`
}

// MCPToolUpdateReq 更新 MCP 工具请求
type MCPToolUpdateReq struct {
	Label       string `json:"label" validate:"required,max=100"`
	Description string `json:"description" validate:"max=500"`
	Command     string `json:"command" validate:"required,max=500"`
	Args        string `json:"args"`
	Env         string `json:"env"`
	Parameters  string `json:"parameters"`
	Category    string `json:"category" validate:"max=50"`
	Version     string `json:"version" validate:"max=50"`
	Timeout     int    `json:"timeout"`
}

// MCPServersPayload 标准 mcpServers JSON 格式（导入/导出）
type MCPServersPayload struct {
	MCPServers map[string]MCPServerEntry `json:"mcpServers"`
}

// MCPServerEntry mcpServers 中单个服务器条目
type MCPServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}
