package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// Message LLM 对话消息
type Message struct {
	Role             string        `json:"role"`                        // system/user/assistant/tool
	Content          string        `json:"content"`                     // 文本内容
	ContentParts     []ContentPart `json:"content_parts,omitempty"`     // 多模态内容（非空时优先于 Content）
	ReasoningContent string        `json:"reasoning_content,omitempty"` // 推理/思考内容（多轮对话需回传）
	ToolCalls        []ToolCall    `json:"tool_calls,omitempty"`        // assistant 返回的工具调用
	ToolCallID       string        `json:"tool_call_id,omitempty"`      // tool 消息的调用 ID
	SystemBlocks     []SystemBlock `json:"-"`                           // 结构化系统提示词（仅 system 角色消息使用，Claude 分块缓存）
}

// ContentPart 多模态内容块
type ContentPart struct {
	Type      string `json:"type"`                 // "text" | "image"
	Text      string `json:"text,omitempty"`       // type=text 时的文本内容
	MediaType string `json:"media_type,omitempty"` // type=image 时的 MIME 类型（image/png, image/jpeg 等）
	Data      string `json:"data,omitempty"`       // type=image 时的 base64 编码数据
	Path      string `json:"path,omitempty"`       // type=image 时的文件路径（相对于工作区，用于前端展示）
}

// SystemBlock 系统提示词分块（供支持分块缓存的 API 使用）
type SystemBlock struct {
	Label   string // 块标签
	Content string // 块内容
}

// ToolCall 工具调用请求
type ToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串
}

// ToolDef 工具定义（发送给 LLM）
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// ChatOptions 对话请求选项
type ChatOptions struct {
	Model           string    `json:"model"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	Temperature     *float64  `json:"temperature,omitempty"`
	Tools           []ToolDef `json:"tools,omitempty"`
	Stream          bool      `json:"stream,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"` // 思考等级：空=不启用，low/medium/high=启用
}

// Response 非流式完整响应
type Response struct {
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"` // 推理/思考内容
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	FinishReason     string     `json:"finish_reason"`
	Usage            Usage      `json:"usage"`
}

// Usage token 用量统计
type Usage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	TotalTokens         int `json:"total_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens,omitempty"`
	CacheReadTokens     int `json:"cache_read_tokens,omitempty"`
}

// Chunk 流式响应片段
type Chunk struct {
	Content      string     `json:"content,omitempty"`
	Reasoning    string     `json:"reasoning,omitempty"` // 推理/思考内容（DeepSeek reasoning_content 等）
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
	FinishReason string     `json:"finish_reason,omitempty"`
	Usage        *Usage     `json:"usage,omitempty"`
	Done         bool       `json:"done"`
	Error        error      `json:"-"`
}

// ProviderConfig LLM 供应商连接配置
type ProviderConfig struct {
	BaseURL    string
	APIKey     string
	Timeout    int // 秒
	MaxRetries int
}

// APIError 带 HTTP 状态码的 API 错误（用于区分 429 等特定错误）
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API 返回错误 %d: %s", e.StatusCode, e.Message)
}

// IsRateLimited 判断是否为 429 限速错误
func IsRateLimited(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429
	}
	return false
}

// ErrIdleTimeout 流式读取空闲超时错误
var ErrIdleTimeout = fmt.Errorf("SSE 流空闲超时")

// idleTimeoutReader 包装 io.ReadCloser，连续 timeout 时间无数据到达则强制关闭
// 用于 SSE 流式读取：允许长时间持续接收数据，但防止连接无声挂起
type idleTimeoutReader struct {
	rc        io.ReadCloser
	timeout   time.Duration
	timer     *time.Timer
	closeOnce sync.Once
	timedOut  bool // 标记是否因超时关闭
}

// newIdleTimeoutReader 创建空闲超时读取器
func newIdleTimeoutReader(rc io.ReadCloser, timeout time.Duration) *idleTimeoutReader {
	r := &idleTimeoutReader{rc: rc, timeout: timeout}
	r.timer = time.AfterFunc(timeout, func() {
		r.timedOut = true
		r.closeOnce.Do(func() { rc.Close() })
	})
	return r
}

func (r *idleTimeoutReader) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	if n > 0 {
		r.timer.Reset(r.timeout) // 收到数据，重置计时
	}
	if err != nil && r.timedOut {
		return n, ErrIdleTimeout // 用明确的超时错误替代底层的 "use of closed network connection"
	}
	return n, err
}

func (r *idleTimeoutReader) Close() error {
	r.timer.Stop()
	r.closeOnce.Do(func() { r.rc.Close() })
	return nil
}
