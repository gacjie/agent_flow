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
)

// OpenAIResponsesClient OpenAI Responses API 客户端（/responses 端点）
type OpenAIResponsesClient struct {
	cfg               ProviderConfig
	httpClient        *http.Client
	timeout           time.Duration
	streamIdleTimeout time.Duration
}

// NewOpenAIResponsesClient 创建 OpenAI Responses API 客户端
func NewOpenAIResponsesClient(cfg ProviderConfig) *OpenAIResponsesClient {
	timeout := 120
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	baseDur := time.Duration(timeout) * time.Second
	return &OpenAIResponsesClient{
		cfg:               cfg,
		timeout:           baseDur,
		streamIdleTimeout: baseDur * 5 / 2,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: baseDur,
			},
		},
	}
}

// ---- Responses API 请求/响应结构 ----

type responsesRequest struct {
	Model          string            `json:"model"`
	Input          json.RawMessage   `json:"input"`
	Tools          []responsesTool   `json:"tools,omitempty"`
	MaxOutputTokens int              `json:"max_output_tokens,omitempty"`
	Temperature    *float64          `json:"temperature,omitempty"`
	Stream         bool              `json:"stream,omitempty"`
	Reasoning      *responsesReason  `json:"reasoning,omitempty"`
}

type responsesReason struct {
	Effort string `json:"effort"`
}

type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// 响应结构
type responsesResponse struct {
	ID     string              `json:"id"`
	Output []responsesOutItem  `json:"output"`
	Usage  responsesUsage      `json:"usage"`
	Status string              `json:"status"`
	Error  *responsesError     `json:"error,omitempty"`
}

type responsesOutItem struct {
	Type      string                `json:"type"`
	ID        string                `json:"id,omitempty"`
	Role      string                `json:"role,omitempty"`
	Status    string                `json:"status,omitempty"`
	Content   []responsesContent    `json:"content,omitempty"`
	Name      string                `json:"name,omitempty"`
	Arguments string                `json:"arguments,omitempty"`
	Summary   []responsesContent    `json:"summary,omitempty"`
}

type responsesContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type responsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type responsesError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ---- 接口实现 ----

func (c *OpenAIResponsesClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error) {
	opts.Stream = false
	body, err := c.buildRequest(messages, opts)
	if err != nil {
		return nil, err
	}

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
		slog.Error("OpenAI Responses API 请求失败", "status", resp.StatusCode, "response", string(data))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(data)}
	}

	var rResp responsesResponse
	if err := json.Unmarshal(data, &rResp); err != nil {
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("解析响应失败（请检查 BaseURL 是否正确）: %s, 原始响应: %s", err.Error(), preview)
	}

	if rResp.Error != nil {
		return nil, fmt.Errorf("API 错误: %s", rResp.Error.Message)
	}

	return c.convertResponse(&rResp), nil
}

func (c *OpenAIResponsesClient) ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (<-chan Chunk, error) {
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
		slog.Error("OpenAI Responses API 流式请求失败", "status", resp.StatusCode, "response", string(data))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(data)}
	}

	resp.Body = newIdleTimeoutReader(resp.Body, c.streamIdleTimeout)

	ch := make(chan Chunk, 32)
	go c.readSSE(resp, ch)
	return ch, nil
}

// ---- 内部方法 ----

func (c *OpenAIResponsesClient) buildRequest(messages []Message, opts ChatOptions) ([]byte, error) {
	req := responsesRequest{
		Model:           opts.Model,
		MaxOutputTokens: opts.MaxTokens,
		Temperature:     opts.Temperature,
		Stream:          opts.Stream,
	}

	if opts.ReasoningEffort != "" {
		req.Reasoning = &responsesReason{Effort: opts.ReasoningEffort}
	}

	// 转换消息为 input 数组
	var inputItems []json.RawMessage
	for _, m := range messages {
		switch m.Role {
		case "system", "user":
			item, _ := json.Marshal(map[string]string{"role": m.Role, "content": m.Content})
			inputItems = append(inputItems, item)
		case "assistant":
			if m.Content != "" {
				item, _ := json.Marshal(map[string]string{"role": "assistant", "content": m.Content})
				inputItems = append(inputItems, item)
			}
			for _, tc := range m.ToolCalls {
				args := tc.Arguments
				if !json.Valid([]byte(args)) {
					args = "{}"
				}
				item, _ := json.Marshal(map[string]string{
					"type": "function_call", "id": tc.ID,
					"name": tc.Name, "arguments": args,
				})
				inputItems = append(inputItems, item)
			}
		case "tool":
			item, _ := json.Marshal(map[string]string{
				"type": "function_call_output", "call_id": m.ToolCallID, "output": m.Content,
			})
			inputItems = append(inputItems, item)
		}
	}

	inputJSON, _ := json.Marshal(inputItems)
	req.Input = inputJSON

	for _, t := range opts.Tools {
		req.Tools = append(req.Tools, responsesTool{
			Type: "function", Name: t.Name,
			Description: t.Description, Parameters: t.Parameters,
		})
	}

	return json.Marshal(req)
}

func (c *OpenAIResponsesClient) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/responses"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	return c.httpClient.Do(req)
}

func (c *OpenAIResponsesClient) convertResponse(rResp *responsesResponse) *Response {
	result := &Response{
		Usage: Usage{
			PromptTokens:     rResp.Usage.InputTokens,
			CompletionTokens: rResp.Usage.OutputTokens,
			TotalTokens:      rResp.Usage.TotalTokens,
		},
	}

	for _, item := range rResp.Output {
		switch item.Type {
		case "message":
			for _, c := range item.Content {
				if c.Type == "output_text" {
					result.Content += c.Text
				}
			}
			if result.FinishReason == "" {
				result.FinishReason = "stop"
			}
		case "reasoning":
			for _, c := range item.Content {
				if c.Type == "reasoning_text" {
					result.ReasoningContent += c.Text
				}
			}
		case "function_call":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID: item.ID, Name: item.Name, Arguments: item.Arguments,
			})
			result.FinishReason = "tool_calls"
		}
	}

	if thinking, cleaned := ExtractThinkTag(result.Content); thinking != "" {
		if result.ReasoningContent == "" {
			result.ReasoningContent = thinking
		}
		result.Content = cleaned
	}

	return result
}

func (c *OpenAIResponsesClient) readSSE(resp *http.Response, ch chan<- Chunk) {
	defer resp.Body.Close()
	defer close(ch)

	type toolAccum struct {
		ID   string
		Name string
		Args strings.Builder
	}
	toolCalls := make(map[int]*toolAccum) // key: output_index

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
		case "response.output_item.added":
			var evt struct {
				OutputIndex int `json:"output_index"`
				Item        struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"item"`
			}
			if json.Unmarshal([]byte(data), &evt) == nil && evt.Item.Type == "function_call" {
				toolCalls[evt.OutputIndex] = &toolAccum{ID: evt.Item.ID, Name: evt.Item.Name}
			}

		case "response.output_text.delta":
			var evt struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal([]byte(data), &evt) == nil && evt.Delta != "" {
				ch <- Chunk{Content: evt.Delta}
			}

		case "response.reasoning_text.delta":
			var evt struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal([]byte(data), &evt) == nil && evt.Delta != "" {
				ch <- Chunk{Reasoning: evt.Delta}
			}

		case "response.function_call_arguments.delta":
			var evt struct {
				OutputIndex int    `json:"output_index"`
				Delta       string `json:"delta"`
			}
			if json.Unmarshal([]byte(data), &evt) == nil {
				if tc, ok := toolCalls[evt.OutputIndex]; ok {
					tc.Args.WriteString(evt.Delta)
				}
			}

		case "response.output_item.done":
			var evt struct {
				OutputIndex int `json:"output_index"`
				Item        struct {
					Type string `json:"type"`
				} `json:"item"`
			}
			if json.Unmarshal([]byte(data), &evt) == nil && evt.Item.Type == "function_call" {
				if tc, ok := toolCalls[evt.OutputIndex]; ok {
					ch <- Chunk{ToolCalls: []ToolCall{{
						ID: tc.ID, Name: tc.Name, Arguments: tc.Args.String(),
					}}}
					delete(toolCalls, evt.OutputIndex)
				}
			}

		case "response.completed":
			var evt struct {
				Response struct {
					Usage responsesUsage `json:"usage"`
				} `json:"response"`
			}
			if json.Unmarshal([]byte(data), &evt) == nil {
				u := evt.Response.Usage
				if u.TotalTokens > 0 {
					ch <- Chunk{Usage: &Usage{
						PromptTokens: u.InputTokens, CompletionTokens: u.OutputTokens, TotalTokens: u.TotalTokens,
					}}
				}
			}
			ch <- Chunk{Done: true}
			return

		case "error":
			var evt struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal([]byte(data), &evt) == nil && evt.Error.Code == "rate_limit_exceeded" {
				ch <- Chunk{Error: &APIError{StatusCode: 429, Message: evt.Error.Message}, Done: true}
			} else {
				ch <- Chunk{Error: fmt.Errorf("OpenAI Responses SSE 错误: %s", data), Done: true}
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- Chunk{Error: fmt.Errorf("读取 SSE 流失败: %w", err), Done: true}
	}
}
