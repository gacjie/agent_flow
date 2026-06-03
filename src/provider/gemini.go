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

// GeminiClient Google Gemini API 客户端（generateContent / streamGenerateContent）
type GeminiClient struct {
	cfg               ProviderConfig
	httpClient        *http.Client
	timeout           time.Duration
	streamIdleTimeout time.Duration
}

// NewGeminiClient 创建 Gemini 客户端
func NewGeminiClient(cfg ProviderConfig) *GeminiClient {
	timeout := 120
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}
	baseDur := time.Duration(timeout) * time.Second
	return &GeminiClient{
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

// ---- Gemini API 请求/响应结构 ----

type geminiRequest struct {
	SystemInstruction *geminiContent    `json:"system_instruction,omitempty"`
	Contents          []geminiContent   `json:"contents"`
	Tools             []geminiToolGroup `json:"tools,omitempty"`
	GenerationConfig  *geminiGenConfig  `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text             string              `json:"text,omitempty"`
	InlineData       *geminiInlineData   `json:"inlineData,omitempty"`
	FunctionCall     *geminiFunctionCall `json:"functionCall,omitempty"`
	FunctionResponse *geminiFuncResponse `json:"functionResponse,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
	ID   string          `json:"id,omitempty"`
}

type geminiFuncResponse struct {
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
	ID       string          `json:"id,omitempty"`
}

type geminiToolGroup struct {
	FunctionDeclarations []geminiFuncDecl `json:"functionDeclarations,omitempty"`
}

type geminiFuncDecl struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type geminiGenConfig struct {
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate `json:"candidates"`
	UsageMetadata *geminiUsage      `json:"usageMetadata,omitempty"`
	Error         *geminiError      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

type geminiUsage struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// ---- 接口实现 ----

func (c *GeminiClient) Chat(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error) {
	body, err := c.buildRequest(messages, opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.doRequest(ctx, body, opts.Model, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("Gemini API 请求失败", "status", resp.StatusCode, "response", string(data))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(data)}
	}

	var gResp geminiResponse
	if err := json.Unmarshal(data, &gResp); err != nil {
		preview := string(data)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return nil, fmt.Errorf("解析响应失败（请检查 BaseURL 是否正确）: %s, 原始响应: %s", err.Error(), preview)
	}

	if gResp.Error != nil {
		return nil, &APIError{StatusCode: gResp.Error.Code, Message: gResp.Error.Message}
	}

	return c.convertResponse(&gResp), nil
}

func (c *GeminiClient) ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (<-chan Chunk, error) {
	body, err := c.buildRequest(messages, opts)
	if err != nil {
		return nil, err
	}

	resp, err := c.doRequest(ctx, body, opts.Model, true)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		slog.Error("Gemini API 流式请求失败", "status", resp.StatusCode, "response", string(data))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(data)}
	}

	resp.Body = newIdleTimeoutReader(resp.Body, c.streamIdleTimeout)

	ch := make(chan Chunk, 32)
	go c.readSSE(resp, ch)
	return ch, nil
}

// ---- 内部方法 ----

func (c *GeminiClient) buildRequest(messages []Message, opts ChatOptions) ([]byte, error) {
	req := geminiRequest{}

	if opts.MaxTokens > 0 || opts.Temperature != nil {
		req.GenerationConfig = &geminiGenConfig{
			MaxOutputTokens: opts.MaxTokens,
			Temperature:     opts.Temperature,
		}
	}

	// 提取 system 消息
	var systemParts []geminiPart
	var convMessages []Message
	for _, m := range messages {
		if m.Role == "system" {
			systemParts = append(systemParts, geminiPart{Text: m.Content})
		} else {
			convMessages = append(convMessages, m)
		}
	}
	if len(systemParts) > 0 {
		req.SystemInstruction = &geminiContent{Parts: systemParts}
	}

	// 转换消息，连续 tool 消息合并到同一个 user 消息
	var pendingFuncParts []geminiPart

	flushFuncParts := func() {
		if len(pendingFuncParts) == 0 {
			return
		}
		req.Contents = append(req.Contents, geminiContent{Role: "user", Parts: pendingFuncParts})
		pendingFuncParts = nil
	}

	for _, m := range convMessages {
		switch m.Role {
		case "tool":
			respData, _ := json.Marshal(map[string]string{"result": m.Content})
			pendingFuncParts = append(pendingFuncParts, geminiPart{
				FunctionResponse: &geminiFuncResponse{
					Name: m.ToolCallID, Response: respData, ID: m.ToolCallID,
				},
			})
		case "assistant":
			flushFuncParts()
			gc := geminiContent{Role: "model"}
			if m.Content != "" {
				gc.Parts = append(gc.Parts, geminiPart{Text: m.Content})
			}
			for _, tc := range m.ToolCalls {
				args := json.RawMessage(tc.Arguments)
				if len(args) == 0 || !json.Valid(args) {
					args = json.RawMessage(`{}`)
				}
				gc.Parts = append(gc.Parts, geminiPart{
					FunctionCall: &geminiFunctionCall{Name: tc.Name, Args: args, ID: tc.ID},
				})
			}
			if len(gc.Parts) > 0 {
				req.Contents = append(req.Contents, gc)
			}
		default:
			flushFuncParts()
			// 多模态内容：ContentParts 非空时构建多 part
			if len(m.ContentParts) > 0 {
				var parts []geminiPart
				for _, p := range m.ContentParts {
					switch p.Type {
					case "image":
						parts = append(parts, geminiPart{
							InlineData: &geminiInlineData{MimeType: p.MediaType, Data: p.Data},
						})
					default:
						parts = append(parts, geminiPart{Text: p.Text})
					}
				}
				req.Contents = append(req.Contents, geminiContent{Role: "user", Parts: parts})
			} else {
				req.Contents = append(req.Contents, geminiContent{
					Role: "user", Parts: []geminiPart{{Text: m.Content}},
				})
			}
		}
	}
	flushFuncParts()

	// 转换工具定义
	if len(opts.Tools) > 0 {
		var decls []geminiFuncDecl
		for _, t := range opts.Tools {
			decls = append(decls, geminiFuncDecl{
				Name: t.Name, Description: t.Description, Parameters: t.Parameters,
			})
		}
		req.Tools = []geminiToolGroup{{FunctionDeclarations: decls}}
	}

	return json.Marshal(req)
}

func (c *GeminiClient) doRequest(ctx context.Context, body []byte, model string, stream bool) (*http.Response, error) {
	baseURL := strings.TrimRight(c.cfg.BaseURL, "/")
	var url string
	if stream {
		url = fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", baseURL, model)
	} else {
		url = fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, model)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.cfg.APIKey)

	return c.httpClient.Do(req)
}

func (c *GeminiClient) convertResponse(gResp *geminiResponse) *Response {
	result := &Response{}

	if gResp.UsageMetadata != nil {
		result.Usage = Usage{
			PromptTokens:     gResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: gResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gResp.UsageMetadata.TotalTokenCount,
		}
	}

	if len(gResp.Candidates) == 0 {
		return result
	}

	candidate := gResp.Candidates[0]
	result.FinishReason = c.mapFinishReason(candidate.FinishReason)

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			result.Content += part.Text
		}
		if part.FunctionCall != nil {
			args, _ := json.Marshal(part.FunctionCall.Args)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID: part.FunctionCall.ID, Name: part.FunctionCall.Name, Arguments: string(args),
			})
		}
	}

	if len(result.ToolCalls) > 0 && result.FinishReason == "stop" {
		result.FinishReason = "tool_calls"
	}

	if thinking, cleaned := ExtractThinkTag(result.Content); thinking != "" {
		if result.ReasoningContent == "" {
			result.ReasoningContent = thinking
		}
		result.Content = cleaned
	}

	return result
}

func (c *GeminiClient) mapFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	default:
		return "stop"
	}
}

func (c *GeminiClient) readSSE(resp *http.Response, ch chan<- Chunk) {
	defer resp.Body.Close()
	defer close(ch)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var gResp geminiResponse
		if err := json.Unmarshal([]byte(data), &gResp); err != nil {
			continue
		}

		if gResp.Error != nil {
			if gResp.Error.Code == 429 {
				ch <- Chunk{Error: &APIError{StatusCode: 429, Message: gResp.Error.Message}, Done: true}
			} else {
				ch <- Chunk{Error: fmt.Errorf("Gemini API 错误: %s", gResp.Error.Message), Done: true}
			}
			return
		}

		if len(gResp.Candidates) > 0 {
			candidate := gResp.Candidates[0]
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					ch <- Chunk{Content: part.Text}
				}
				if part.FunctionCall != nil {
					args, _ := json.Marshal(part.FunctionCall.Args)
					ch <- Chunk{ToolCalls: []ToolCall{{
						ID: part.FunctionCall.ID, Name: part.FunctionCall.Name, Arguments: string(args),
					}}}
				}
			}
			if candidate.FinishReason != "" {
				ch <- Chunk{FinishReason: c.mapFinishReason(candidate.FinishReason)}
			}
		}

		if gResp.UsageMetadata != nil && gResp.UsageMetadata.TotalTokenCount > 0 {
			ch <- Chunk{Usage: &Usage{
				PromptTokens:     gResp.UsageMetadata.PromptTokenCount,
				CompletionTokens: gResp.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      gResp.UsageMetadata.TotalTokenCount,
			}}
		}
	}

	if err := scanner.Err(); err != nil {
		ch <- Chunk{Error: fmt.Errorf("读取 SSE 流失败: %w", err), Done: true}
		return
	}

	ch <- Chunk{Done: true}
}
