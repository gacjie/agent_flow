package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"agent_flow/src/common"
)

// OpenAIClient OpenAI 兼容协议客户端（覆盖 OpenAI/DeepSeek/通义千问等）
type OpenAIClient struct {
	cfg             ProviderConfig
	httpClient      *http.Client
	timeout         time.Duration // 非流式总超时
	streamIdleTimeout time.Duration // 流式空闲超时（更长，适应模型思考阶段）
}

// NewOpenAIClient 创建 OpenAI 兼容客户端
func NewOpenAIClient(cfg ProviderConfig) *OpenAIClient {
	timeout := 120
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	baseDur := time.Duration(timeout) * time.Second
	streamIdle := baseDur * 5 / 2
	if cfg.StreamIdleTimeout > 0 {
		streamIdle = time.Duration(cfg.StreamIdleTimeout) * time.Second
	}
	return &OpenAIClient{
		cfg:               cfg,
		timeout:           baseDur,
		streamIdleTimeout: streamIdle,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: baseDur,
			},
		},
	}
}

func (c *OpenAIClient) DoubleStreamIdleTimeout() {
	c.streamIdleTimeout *= 2
}

// ---- OpenAI API 请求/响应结构 ----

type openaiRequest struct {
	Model           string             `json:"model"`
	Messages        []openaiMessage    `json:"messages"`
	MaxTokens       int                `json:"max_tokens,omitempty"`
	Temperature     *float64           `json:"temperature,omitempty"`
	Tools           []openaiTool       `json:"tools,omitempty"`
	Stream          bool               `json:"stream,omitempty"`
	StreamOptions   *openaiStreamOpts  `json:"stream_options,omitempty"`
	ReasoningEffort string             `json:"reasoning_effort,omitempty"`
	Thinking        *openaiThinking    `json:"thinking,omitempty"`
	EnableThinking  *bool              `json:"enable_thinking,omitempty"`
	ReasoningSplit  *bool              `json:"reasoning_split,omitempty"`
}

type openaiStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openaiThinking struct {
	Type string `json:"type"`
}

type openaiMessage struct {
	Role             string           `json:"role"`
	Content          json.RawMessage  `json:"content"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openaiToolCall struct {
	Index    int                `json:"index"`
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openaiToolCallFunc `json:"function"`
}

type openaiToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
	Error   *openaiError   `json:"error,omitempty"`
}

type openaiChoice struct {
	Message      openaiMessage `json:"message"`
	Delta        openaiDelta   `json:"delta"`
	FinishReason string        `json:"finish_reason"`
}

type openaiDelta struct {
	Role             string           `json:"role,omitempty"`
	Content          string           `json:"content,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"` // DeepSeek 等模型的推理内容
	ToolCalls        []openaiToolCall `json:"tool_calls,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// ---- 接口实现 ----

// Chat 非流式对话
func (c *OpenAIClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error) {
	opts.Stream = false
	body, err := c.buildRequest(messages, opts)
	if err != nil {
		return nil, err
	}

	// 非流式请求使用 context.WithTimeout 控制总超时
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("OpenAI API 请求失败", "status", resp.StatusCode, "response", string(data), "request", string(body))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(data)}
	}

	var oResp openaiResponse
	if err := json.Unmarshal(data, &oResp); err != nil {
		// 检测是否为 SSE 流式格式（某些代理 API 即使非流式请求也返回 SSE）
		if bytes.HasPrefix(bytes.TrimSpace(data), []byte("data: ")) {
			errMsg := extractSSEError(data)
			if errMsg != "" {
				return nil, fmt.Errorf("API 错误（SSE 格式）: %s", errMsg)
			}
		}
		// 显示原始响应前 200 字符，便于排查 BaseURL 配置错误
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("解析响应失败（请检查 BaseURL 是否正确）: %s, 原始响应: %s", err.Error(), preview)
	}

	if oResp.Error != nil {
		return nil, fmt.Errorf("API 错误: %s", oResp.Error.Message)
	}

	if len(oResp.Choices) == 0 {
		return nil, fmt.Errorf("API 返回空响应")
	}

	choice := oResp.Choices[0]
	// 从 json.RawMessage 提取文本内容（响应中 content 始终为字符串）
	var contentStr string
	_ = json.Unmarshal(choice.Message.Content, &contentStr)
	result := &Response{
		Content:          contentStr,
		ReasoningContent: choice.Message.ReasoningContent,
		FinishReason:     choice.FinishReason,
		Usage: Usage{
			PromptTokens:     oResp.Usage.PromptTokens,
			CompletionTokens: oResp.Usage.CompletionTokens,
			TotalTokens:      oResp.Usage.TotalTokens,
		},
	}

	for _, tc := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	// 自动提取 <think> 标签内容
	if thinking, cleaned := ExtractThinkTag(result.Content); thinking != "" {
		if result.ReasoningContent == "" {
			result.ReasoningContent = thinking
		}
		result.Content = cleaned
	}

	return result, nil
}

// ChatStream 流式对话
func (c *OpenAIClient) ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (<-chan Chunk, error) {
	opts.Stream = true
	body, err := c.buildRequest(messages, opts)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		slog.Error("OpenAI API 流式请求失败", "status", resp.StatusCode, "response", string(data), "request", string(body))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(data)}
	}

	// 流式不设总超时，改用空闲超时读取器防止连接无声挂起
	resp.Body = newIdleTimeoutReader(resp.Body, c.streamIdleTimeout)

	ch := make(chan Chunk, 32)
	go c.readSSE(resp, ch)
	return ch, nil
}

// ---- 内部方法 ----

func (c *OpenAIClient) buildRequest(messages []Message, opts ChatOptions) ([]byte, error) {
	req := openaiRequest{
		Model:       opts.Model,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		Stream:      opts.Stream,
	}

	if opts.Stream {
		req.StreamOptions = &openaiStreamOpts{IncludeUsage: true}
	}

	// 转换消息
	for _, m := range messages {
		om := openaiMessage{
			Role:             m.Role,
			ReasoningContent: m.ReasoningContent,
			ToolCallID:       m.ToolCallID,
		}
		// 多模态内容：ContentParts 非空时构建数组格式
		if len(m.ContentParts) > 0 && (m.Role == "user" || m.Role == "tool") {
			var parts []map[string]interface{}
			for _, p := range m.ContentParts {
				switch p.Type {
				case "image":
					parts = append(parts, map[string]interface{}{
						"type": "image_url",
						"image_url": map[string]string{
							"url": "data:" + p.MediaType + ";base64," + p.Data,
						},
					})
				default:
					parts = append(parts, map[string]interface{}{
						"type": "text",
						"text": p.Text,
					})
				}
			}
			om.Content, _ = json.Marshal(parts)
		} else {
			om.Content, _ = json.Marshal(m.Content)
		}
		for _, tc := range m.ToolCalls {
			args := tc.Arguments
			if !json.Valid([]byte(args)) {
				args = "{}"
			}
			om.ToolCalls = append(om.ToolCalls, openaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: openaiToolCallFunc{
					Name:      tc.Name,
					Arguments: args,
				},
			})
		}
		// DeepSeek 思考模式要求所有 assistant 消息都携带 reasoning_content 字段
		if opts.ReasoningEffort != "" && om.Role == "assistant" && om.ReasoningContent == "" {
			om.ReasoningContent = "."
		}
		req.Messages = append(req.Messages, om)
	}

	// 转换工具定义
	for _, t := range opts.Tools {
		req.Tools = append(req.Tools, openaiTool{
			Type: "function",
			Function: openaiFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	// 思考模式：全量发送所有厂商差异参数，API 自动忽略不认识的字段
	if opts.ReasoningEffort != "" {
		req.ReasoningEffort = opts.ReasoningEffort
		req.Thinking = &openaiThinking{Type: "enabled"}
		t := true
		req.EnableThinking = &t
		req.ReasoningSplit = &t
	}

	return json.Marshal(req)
}

func (c *OpenAIClient) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	url := common.BuildAPIURL(c.cfg.BaseURL, "/chat/completions")

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	return c.httpClient.Do(req)
}

func (c *OpenAIClient) readSSE(resp *http.Response, ch chan<- Chunk) {
	defer resp.Body.Close()
	defer close(ch)

	// 累积工具调用参数（流式中 tool_calls 是分片传输的）
	toolCallAccum := make(map[int]*ToolCall)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			// 发送累积的工具调用
			if len(toolCallAccum) > 0 {
				var calls []ToolCall
				for _, tc := range toolCallAccum {
					calls = append(calls, *tc)
				}
				ch <- Chunk{ToolCalls: calls, FinishReason: "tool_calls"}
			}
			ch <- Chunk{Done: true}
			return
		}

		var oResp openaiResponse
		if err := json.Unmarshal([]byte(data), &oResp); err != nil {
			continue
		}

		if len(oResp.Choices) == 0 {
			// 可能是最后的 usage 消息
			if oResp.Usage.TotalTokens > 0 {
				ch <- Chunk{
					Usage: &Usage{
						PromptTokens:     oResp.Usage.PromptTokens,
						CompletionTokens: oResp.Usage.CompletionTokens,
						TotalTokens:      oResp.Usage.TotalTokens,
					},
				}
			}
			continue
		}

		choice := oResp.Choices[0]
		delta := choice.Delta

		// 文本内容
		if delta.Content != "" {
			ch <- Chunk{Content: delta.Content}
		}

		// 推理/思考内容（DeepSeek reasoning_content 等）
		if delta.ReasoningContent != "" {
			ch <- Chunk{Reasoning: delta.ReasoningContent}
		}

		// 工具调用（流式中分片传输，需要累积）
		for _, tc := range delta.ToolCalls {
			if tc.ID != "" {
				// 新的工具调用开始（ID 只在第一个 chunk 出现）
				toolCallAccum[tc.Index] = &ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
				}
			}
			if existing, ok := toolCallAccum[tc.Index]; ok {
				if tc.Function.Name != "" {
					existing.Name = tc.Function.Name
				}
				existing.Arguments += tc.Function.Arguments
			}
		}

		// 完成原因
		if choice.FinishReason != "" && choice.FinishReason != "tool_calls" {
			ch <- Chunk{FinishReason: choice.FinishReason}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- Chunk{Error: fmt.Errorf("读取 SSE 流失败: %w", err), Done: true}
	}
}

// extractSSEError 从 SSE 格式的响应中提取错误信息
func extractSSEError(data []byte) string {
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		payload := bytes.TrimPrefix(line, []byte("data: "))
		var obj struct {
			Error *openaiError `json:"error"`
		}
		if json.Unmarshal(payload, &obj) == nil && obj.Error != nil {
			return obj.Error.Message
		}
	}
	return ""
}

// thinkTagRe 匹配 <think>...</think> 标签（含换行）
var thinkTagRe = regexp.MustCompile(`(?s)<think>(.*?)</think>`)

// ExtractThinkTag 从 content 中提取 <think>...</think> 标签内容
// 返回提取的思考内容和清理后的 content；无匹配时原样返回
func ExtractThinkTag(content string) (thinking, cleaned string) {
	matches := thinkTagRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return "", content
	}
	var parts []string
	for _, m := range matches {
		parts = append(parts, strings.TrimSpace(m[1]))
	}
	thinking = strings.Join(parts, "\n")
	cleaned = strings.TrimSpace(thinkTagRe.ReplaceAllString(content, ""))
	return thinking, cleaned
}
