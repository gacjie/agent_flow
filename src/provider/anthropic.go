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
	"strings"
	"time"

	"agent_flow/src/common"
)

// ClaudeClient Anthropic Claude API 客户端
type ClaudeClient struct {
	cfg               ProviderConfig
	httpClient        *http.Client
	timeout           time.Duration // 非流式总超时
	streamIdleTimeout time.Duration // 流式空闲超时（更长，适应模型思考阶段）
}

// NewClaudeClient 创建 Claude 客户端
func NewClaudeClient(cfg ProviderConfig) *ClaudeClient {
	timeout := 120
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	baseDur := time.Duration(timeout) * time.Second
	streamIdle := baseDur * 5 / 2
	if cfg.StreamIdleTimeout > 0 {
		streamIdle = time.Duration(cfg.StreamIdleTimeout) * time.Second
	}
	return &ClaudeClient{
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

func (c *ClaudeClient) DoubleStreamIdleTimeout() {
	c.streamIdleTimeout *= 2
}

// ---- Claude API 请求/响应结构 ----

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    json.RawMessage `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
	Tools     []claudeTool    `json:"tools,omitempty"`
	Stream    bool            `json:"stream,omitempty"`
}

type claudeMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string 或 []contentBlock
}

type contentBlock struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name,omitempty"`
	Input        json.RawMessage `json:"input,omitempty"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`
	Content      json.RawMessage `json:"content,omitempty"` // tool_result 使用数组格式
	IsError      bool            `json:"is_error,omitempty"`
	CacheControl *cacheControl   `json:"cache_control,omitempty"`
}

type claudeTool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	CacheControl *cacheControl   `json:"cache_control,omitempty"`
}

// Claude prompt caching 相关类型
type claudeSystemBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text"`
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type claudeResponse struct {
	Content      []contentBlock `json:"content"`
	StopReason   string         `json:"stop_reason"`
	Usage        claudeUsage    `json:"usage"`
	Error        *claudeError   `json:"error,omitempty"`
}

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type claudeError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Claude SSE 事件类型
type claudeSSEEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`

	// message_start 事件
	Message *claudeResponse `json:"message,omitempty"`

	// content_block_start 事件
	ContentBlock *contentBlock `json:"content_block,omitempty"`

	// message_delta 事件
	Usage *claudeUsage `json:"usage,omitempty"`
}

type claudeDelta struct {
	Type         string `json:"type,omitempty"`
	Text         string `json:"text,omitempty"`
	Thinking     string `json:"thinking,omitempty"`   // thinking_delta 的推理内容
	PartialJSON  string `json:"partial_json,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
}

// ---- 接口实现 ----

// Chat 非流式对话
func (c *ClaudeClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error) {
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
		slog.Error("Claude API 请求失败", "status", resp.StatusCode, "response", string(data), "request", string(body))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(data)}
	}

	var cResp claudeResponse
	if err := json.Unmarshal(data, &cResp); err != nil {
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("解析响应失败（请检查 BaseURL 是否正确）: %s, 原始响应: %s", err.Error(), preview)
	}

	if cResp.Error != nil {
		return nil, fmt.Errorf("API 错误: %s", cResp.Error.Message)
	}

	return c.convertResponse(&cResp), nil
}

// ChatStream 流式对话
func (c *ClaudeClient) ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (<-chan Chunk, error) {
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
		slog.Error("Claude API 流式请求失败", "status", resp.StatusCode, "response", string(data), "request", string(body))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(data)}
	}

	// 流式不设总超时，改用空闲超时读取器防止连接无声挂起
	resp.Body = newIdleTimeoutReader(resp.Body, c.streamIdleTimeout)

	ch := make(chan Chunk, 32)
	go c.readSSE(resp, ch)
	return ch, nil
}

// ---- 内部方法 ----

func (c *ClaudeClient) buildRequest(messages []Message, opts ChatOptions) ([]byte, error) {
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	req := claudeRequest{
		Model:     opts.Model,
		MaxTokens: maxTokens,
		Stream:    opts.Stream,
	}

	// 提取 system 消息
	var systemText string
	var systemBlocks []SystemBlock
	var convMessages []Message
	for _, m := range messages {
		if m.Role == "system" {
			if systemText != "" {
				systemText += "\n" + m.Content
			} else {
				systemText = m.Content
			}
			if len(m.SystemBlocks) > 0 {
				systemBlocks = m.SystemBlocks
			}
		} else {
			convMessages = append(convMessages, m)
		}
	}

	// 构建 system 字段：有 SystemBlocks 时使用分块格式（支持 cache_control），否则用字符串
	if len(systemBlocks) > 0 {
		blocks := make([]claudeSystemBlock, len(systemBlocks))
		for i, sb := range systemBlocks {
			blocks[i] = claudeSystemBlock{
				Type: "text",
				Text: fmt.Sprintf("<%s>\n%s\n</%s>", sb.Label, sb.Content, sb.Label),
			}
		}
		// 在最后一个块上标记 cache_control（整个 system 都是稳定内容）
		if len(blocks) > 0 {
			blocks[len(blocks)-1].CacheControl = &cacheControl{Type: "ephemeral"}
		}
		data, _ := json.Marshal(blocks)
		req.System = data
	} else if systemText != "" {
		data, _ := json.Marshal(systemText)
		req.System = data
	}

	// 转换消息，连续的 tool 消息合并到同一个 user 消息（Claude API 不允许连续相同角色）
	var pendingToolBlocks []contentBlock

	flushTools := func() {
		if len(pendingToolBlocks) == 0 {
			return
		}
		data, _ := json.Marshal(pendingToolBlocks)
		req.Messages = append(req.Messages, claudeMessage{Role: "user", Content: data})
		pendingToolBlocks = nil
	}

	for _, m := range convMessages {
		switch m.Role {
		case "tool":
			// Claude API 要求 tool_result 的 content 为内容块数组格式
			textBlock := []map[string]string{{"type": "text", "text": m.Content}}
			contentJSON, _ := json.Marshal(textBlock)
			pendingToolBlocks = append(pendingToolBlocks, contentBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   contentJSON,
			})
		case "assistant":
			flushTools()
			cm := claudeMessage{Role: "assistant"}
			if len(m.ToolCalls) > 0 {
				var blocks []contentBlock
				if m.Content != "" {
					blocks = append(blocks, contentBlock{Type: "text", Text: m.Content})
				}
				for _, tc := range m.ToolCalls {
					input := json.RawMessage(tc.Arguments)
					if len(input) == 0 || !json.Valid(input) {
						input = json.RawMessage(`{}`)
					}
					blocks = append(blocks, contentBlock{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Name,
						Input: input,
					})
				}
				data, _ := json.Marshal(blocks)
				cm.Content = data
			} else {
				data, _ := json.Marshal(m.Content)
				cm.Content = data
			}
			req.Messages = append(req.Messages, cm)
		default:
			flushTools()
			cm := claudeMessage{Role: m.Role}
			// 多模态内容：ContentParts 非空时构建内容块数组
			if len(m.ContentParts) > 0 {
				var blocks []map[string]interface{}
				for _, p := range m.ContentParts {
					switch p.Type {
					case "image":
						blocks = append(blocks, map[string]interface{}{
							"type": "image",
							"source": map[string]string{
								"type":       "base64",
								"media_type": p.MediaType,
								"data":       p.Data,
							},
						})
					default:
						blocks = append(blocks, map[string]interface{}{
							"type": "text",
							"text": p.Text,
						})
					}
				}
				cm.Content, _ = json.Marshal(blocks)
			} else {
				data, _ := json.Marshal(m.Content)
				cm.Content = data
			}
			req.Messages = append(req.Messages, cm)
		}
	}
	flushTools()

	// 转换工具定义
	for _, t := range opts.Tools {
		req.Tools = append(req.Tools, claudeTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	// 在最后一个工具上标记 cache_control（工具定义完全稳定）
	if len(req.Tools) > 0 {
		req.Tools[len(req.Tools)-1].CacheControl = &cacheControl{Type: "ephemeral"}
	}

	// 在倒数第二条消息上标记 cache_control（缓存整个对话历史前缀）
	if len(req.Messages) >= 2 {
		idx := len(req.Messages) - 2
		msg := &req.Messages[idx]
		// 将字符串格式的 content 转为带 cache_control 的内容块数组
		var str string
		if json.Unmarshal(msg.Content, &str) == nil {
			block := []contentBlock{{
				Type:         "text",
				Text:         str,
				CacheControl: &cacheControl{Type: "ephemeral"},
			}}
			msg.Content, _ = json.Marshal(block)
		} else {
			// content 已经是数组格式，在最后一个块上加 cache_control
			var blocks []contentBlock
			if json.Unmarshal(msg.Content, &blocks) == nil && len(blocks) > 0 {
				blocks[len(blocks)-1].CacheControl = &cacheControl{Type: "ephemeral"}
				msg.Content, _ = json.Marshal(blocks)
			}
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func (c *ClaudeClient) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	url := common.BuildAPIURL(c.cfg.BaseURL, "/v1/messages")

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	return c.httpClient.Do(req)
}

func (c *ClaudeClient) convertResponse(cResp *claudeResponse) *Response {
	result := &Response{
		FinishReason: cResp.StopReason,
		Usage: Usage{
			PromptTokens:        cResp.Usage.InputTokens,
			CompletionTokens:    cResp.Usage.OutputTokens,
			TotalTokens:         cResp.Usage.InputTokens + cResp.Usage.OutputTokens,
			CacheCreationTokens: cResp.Usage.CacheCreationInputTokens,
			CacheReadTokens:     cResp.Usage.CacheReadInputTokens,
		},
	}

	for _, block := range cResp.Content {
		switch block.Type {
		case "text":
			result.Content += block.Text
		case "thinking":
			result.ReasoningContent += block.Text
		case "tool_use":
			args, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(args),
			})
		}
	}

	// 自动提取 <think> 标签内容
	if thinking, cleaned := ExtractThinkTag(result.Content); thinking != "" {
		if result.ReasoningContent == "" {
			result.ReasoningContent = thinking
		}
		result.Content = cleaned
	}

	return result
}

func (c *ClaudeClient) readSSE(resp *http.Response, ch chan<- Chunk) {
	defer resp.Body.Close()
	defer close(ch)

	// 累积工具调用
	type toolAccum struct {
		ID   string
		Name string
		Args strings.Builder
	}
	var currentTool *toolAccum
	var inputTokens int                // 从 message_start 事件获取
	var cacheCreationTokens int        // 缓存写入 token
	var cacheReadTokens int            // 缓存读取 token

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var eventType string
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		switch eventType {
		case "message_start":
			var evt claudeSSEEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}
			if evt.Message != nil {
				inputTokens = evt.Message.Usage.InputTokens
				cacheCreationTokens = evt.Message.Usage.CacheCreationInputTokens
				cacheReadTokens = evt.Message.Usage.CacheReadInputTokens
			}

		case "content_block_start":
			var evt claudeSSEEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}
			if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
				currentTool = &toolAccum{
					ID:   evt.ContentBlock.ID,
					Name: evt.ContentBlock.Name,
				}
			}

		case "content_block_delta":
			var evt claudeSSEEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}
			var delta claudeDelta
			if err := json.Unmarshal(evt.Delta, &delta); err != nil {
				continue
			}

			switch delta.Type {
			case "text_delta":
				if delta.Text != "" {
					ch <- Chunk{Content: delta.Text}
				}
			case "thinking_delta":
				if delta.Thinking != "" {
					ch <- Chunk{Reasoning: delta.Thinking}
				}
			case "input_json_delta":
				if currentTool != nil {
					currentTool.Args.WriteString(delta.PartialJSON)
				}
			}

		case "content_block_stop":
			if currentTool != nil {
				ch <- Chunk{
					ToolCalls: []ToolCall{{
						ID:        currentTool.ID,
						Name:      currentTool.Name,
						Arguments: currentTool.Args.String(),
					}},
				}
				currentTool = nil
			}

		case "message_delta":
			var evt claudeSSEEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}
			var delta claudeDelta
			if err := json.Unmarshal(evt.Delta, &delta); err != nil {
				continue
			}
			if delta.StopReason != "" {
				ch <- Chunk{FinishReason: delta.StopReason}
			}
			if evt.Usage != nil {
				ch <- Chunk{
					Usage: &Usage{
						PromptTokens:        inputTokens,
						CompletionTokens:    evt.Usage.OutputTokens,
						TotalTokens:         inputTokens + evt.Usage.OutputTokens,
						CacheCreationTokens: cacheCreationTokens,
						CacheReadTokens:     cacheReadTokens,
					},
				}
			}

		case "message_stop":
			ch <- Chunk{Done: true}
			return

		case "error":
			var errData struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(data), &errData) == nil {
				switch errData.Error.Type {
				case "rate_limit_error":
					ch <- Chunk{Error: &APIError{StatusCode: 429, Message: errData.Error.Message}, Done: true}
				case "overloaded_error":
					ch <- Chunk{Error: &APIError{StatusCode: 529, Message: errData.Error.Message}, Done: true}
				default:
					ch <- Chunk{Error: fmt.Errorf("Claude SSE 错误: %s", data), Done: true}
				}
			} else {
				ch <- Chunk{Error: fmt.Errorf("Claude SSE 错误: %s", data), Done: true}
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- Chunk{Error: fmt.Errorf("读取 SSE 流失败: %w", err), Done: true}
	}
}
