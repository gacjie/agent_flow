package tool

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

// MCPToolAdapter MCP 工具适配器（通过持久 MCPClient 执行）
type MCPToolAdapter struct {
	name        string
	description string
	params      json.RawMessage
	serverName  string
	toolName    string
	manager     *MCPManager
}

// Name 工具名称（mcp__{server}__{tool} 格式）
func (a *MCPToolAdapter) Name() string {
	return a.name
}

// Description 工具描述（来自 tools/list）
func (a *MCPToolAdapter) Description() string {
	return a.description
}

// Parameters JSON Schema 参数定义（来自 tools/list 的 inputSchema）
func (a *MCPToolAdapter) Parameters() json.RawMessage {
	if len(a.params) > 0 {
		return a.params
	}
	return json.RawMessage(`{"type":"object","properties":{}}`)
}

// Execute 通过持久 MCPClient 执行工具调用
func (a *MCPToolAdapter) Execute(ctx context.Context, args string) *Result {
	client, ok := a.manager.GetClient(a.serverName)
	if !ok {
		return ErrorResult("MCP 服务器未运行: " + a.serverName)
	}

	var arguments map[string]interface{}
	if args != "" {
		if err := json.Unmarshal([]byte(args), &arguments); err != nil {
			return ErrorResult("参数解析失败: " + err.Error())
		}
	}
	if arguments == nil {
		arguments = make(map[string]interface{})
	}

	callCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	result, err := client.CallTool(callCtx, a.toolName, arguments)
	if err != nil {
		return ErrorResult("MCP 工具调用失败: " + err.Error())
	}

	var sb strings.Builder
	for _, c := range result.Content {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		switch c.Type {
		case "text":
			sb.WriteString(c.Text)
		case "image":
			sb.WriteString("[image: " + c.MimeType + "]")
		case "resource":
			sb.WriteString(c.Text)
		default:
			slog.Debug("MCP: 不支持的内容类型", "type", c.Type, "server", a.serverName, "tool", a.toolName)
			sb.WriteString("[unsupported: " + c.Type + "]")
		}
	}

	if result.IsError {
		return ErrorResult(sb.String())
	}
	return SuccessResult(sb.String())
}

// parseCommandLine 解析命令行字符串为参数列表，支持单引号、双引号和反斜杠转义
func parseCommandLine(command string) []string {
	var parts []string
	var cur []byte
	inSingle := false
	inDouble := false

	for i := 0; i < len(command); i++ {
		ch := command[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '\\' && !inSingle && i+1 < len(command):
			i++
			cur = append(cur, command[i])
		case (ch == ' ' || ch == '\t') && !inSingle && !inDouble:
			if len(cur) > 0 {
				parts = append(parts, string(cur))
				cur = cur[:0]
			}
		default:
			cur = append(cur, ch)
		}
	}
	if len(cur) > 0 {
		parts = append(parts, string(cur))
	}
	return parts
}
