package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// limitedBuffer 限制大小的缓冲区（用于捕获 stderr，防止内存泄漏）
type limitedBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func newLimitedBuffer(max int) *limitedBuffer {
	return &limitedBuffer{max: max}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := b.max - len(b.buf)
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.buf))
}

// JSON-RPC 2.0 协议结构体

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int64      `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP 协议数据结构

// MCPToolDef tools/list 返回的工具定义
type MCPToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// MCPCallResult tools/call 返回的结果
type MCPCallResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError"`
}

// MCPContent MCP 内容块
type MCPContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

// MCPClient MCP JSON-RPC 2.0 客户端（stdio 传输）
type MCPClient struct {
	command        string
	args           []string
	env            []string
	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdout         *bufio.Scanner
	stderrBuf      *limitedBuffer
	mu             sync.Mutex
	nextID         atomic.Int64
	pending        sync.Map
	done           chan struct{}
	closeOnce      sync.Once
	err            error
	onToolsChanged func()
}

const mcpProtocolVersion = "2025-03-26"

// NewMCPClient 创建 MCP 客户端（不启动进程）
func NewMCPClient(command string, args []string, env []string) *MCPClient {
	return &MCPClient{
		command:   command,
		args:      args,
		env:       env,
		stderrBuf: newLimitedBuffer(8 * 1024),
		done:      make(chan struct{}),
	}
}

// OnToolsChanged 设置工具列表变更回调（收到 notifications/tools/list_changed 时触发）
func (c *MCPClient) OnToolsChanged(fn func()) {
	c.onToolsChanged = fn
}

// Start 启动 MCP 服务器进程并完成 initialize 握手
func (c *MCPClient) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.command, c.args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建 stdin 管道失败: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}

	cmd.Stderr = c.stderrBuf
	if len(c.env) > 0 {
		cmd.Env = mergeEnv(os.Environ(), c.env)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 MCP 服务器失败: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewScanner(stdout)
	c.stdout.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	go c.readLoop()

	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := c.initialize(initCtx); err != nil {
		c.Close()
		stderr := c.stderrBuf.String()
		if stderr != "" {
			return fmt.Errorf("MCP 握手失败: %w\nstderr: %s", err, stderr)
		}
		return fmt.Errorf("MCP 握手失败: %w", err)
	}

	return nil
}

// initialize 执行 MCP initialize 握手
func (c *MCPClient) initialize(ctx context.Context) error {
	params := map[string]interface{}{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "AgentFlow",
			"version": "1.0.0",
		},
	}

	resp, err := c.send(ctx, "initialize", params)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("initialize 错误: [%d] %s", resp.Error.Code, resp.Error.Message)
	}

	return c.notify("notifications/initialized", nil)
}

// ListTools 调用 tools/list 获取服务器提供的所有工具定义（支持分页）
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPToolDef, error) {
	var allTools []MCPToolDef
	var cursor string
	const maxPages = 20

	for page := 0; page < maxPages; page++ {
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}

		resp, err := c.send(ctx, "tools/list", params)
		if err != nil {
			return nil, err
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("tools/list 错误: [%d] %s", resp.Error.Code, resp.Error.Message)
		}

		var result struct {
			Tools      []MCPToolDef `json:"tools"`
			NextCursor string       `json:"nextCursor"`
		}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return nil, fmt.Errorf("解析 tools/list 响应失败: %w", err)
		}

		allTools = append(allTools, result.Tools...)

		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return allTools, nil
}

// CallTool 调用 tools/call 执行工具（线程安全）
func (c *MCPClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*MCPCallResult, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": arguments,
	}

	resp, err := c.send(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("tools/call 错误: [%d] %s", resp.Error.Code, resp.Error.Message)
	}

	var result MCPCallResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析 tools/call 响应失败: %w", err)
	}

	return &result, nil
}

// Close 优雅关闭 MCP 服务器
func (c *MCPClient) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		if c.stdin != nil {
			c.stdin.Close()
		}

		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		waitDone := make(chan error, 1)
		go func() {
			if c.cmd != nil && c.cmd.Process != nil {
				waitDone <- c.cmd.Wait()
			} else {
				waitDone <- nil
			}
		}()

		select {
		case closeErr = <-waitDone:
		case <-timer.C:
			if c.cmd != nil && c.cmd.Process != nil {
				c.cmd.Process.Kill()
			}
			closeErr = errors.New("MCP 服务器关闭超时，已强制终止")
		}
	})
	return closeErr
}

// Alive 检查进程是否仍在运行
func (c *MCPClient) Alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

// send 发送 JSON-RPC 请求并等待响应
func (c *MCPClient) send(ctx context.Context, method string, params interface{}) (*jsonrpcResponse, error) {
	select {
	case <-c.done:
		return nil, fmt.Errorf("MCP 服务器已退出: %v", c.err)
	default:
	}

	id := c.nextID.Add(1)
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  method,
		Params:  params,
	}

	ch := make(chan *jsonrpcResponse, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	if err := c.writeMessage(req); err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	select {
	case resp := <-ch:
		if resp == nil {
			stderr := c.stderrBuf.String()
			if stderr != "" {
				return nil, fmt.Errorf("MCP 服务器已断开连接\nstderr: %s", stderr)
			}
			return nil, fmt.Errorf("MCP 服务器已断开连接")
		}
		return resp, nil
	case <-c.done:
		stderr := c.stderrBuf.String()
		if stderr != "" {
			return nil, fmt.Errorf("MCP 服务器已退出: %v\nstderr: %s", c.err, stderr)
		}
		return nil, fmt.Errorf("MCP 服务器已退出: %v", c.err)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// notify 发送 JSON-RPC 通知（无 ID，不等待响应）
func (c *MCPClient) notify(method string, params interface{}) error {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.writeMessage(req)
}

// writeMessage 序列化并写入一条 JSON-RPC 消息（换行分隔）
func (c *MCPClient) writeMessage(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.stdin.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

// readLoop 后台读取 stdout，解析 JSON-RPC 响应并分发
func (c *MCPClient) readLoop() {
	defer func() {
		c.pending.Range(func(key, value interface{}) bool {
			if ch, ok := value.(chan *jsonrpcResponse); ok {
				close(ch)
			}
			c.pending.Delete(key)
			return true
		})
		close(c.done)
	}()

	for c.stdout.Scan() {
		line := strings.TrimSpace(c.stdout.Text())
		if line == "" {
			continue
		}

		var resp jsonrpcResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			slog.Debug("MCP: 忽略非 JSON 行", "line", line[:min(len(line), 100)])
			continue
		}

		if resp.ID == nil {
			if resp.Method == "notifications/tools/list_changed" && c.onToolsChanged != nil {
				go c.onToolsChanged()
			}
			continue
		}

		if ch, ok := c.pending.LoadAndDelete(*resp.ID); ok {
			ch.(chan *jsonrpcResponse) <- &resp
		}
	}

	if err := c.stdout.Err(); err != nil {
		c.err = err
	} else {
		stderr := c.stderrBuf.String()
		if stderr != "" {
			c.err = fmt.Errorf("stdout EOF\nstderr: %s", stderr)
		} else {
			c.err = errors.New("stdout EOF")
		}
	}
}

// mergeEnv 合并环境变量，extra 中的同名变量覆盖 base
func mergeEnv(base []string, extra []string) []string {
	envMap := make(map[string]string, len(base))
	for _, e := range base {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMap[k] = v
		}
	}
	for _, e := range extra {
		if k, v, ok := strings.Cut(e, "="); ok {
			envMap[k] = v
		}
	}
	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, k+"="+v)
	}
	return result
}
