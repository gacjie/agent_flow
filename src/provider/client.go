package provider

import (
	"context"
	"fmt"
	"strings"
)

// Client LLM 客户端接口
type Client interface {
	// Chat 非流式对话，返回完整响应
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (*Response, error)
	// ChatStream 流式对话，返回 chunk 通道
	ChatStream(ctx context.Context, messages []Message, opts ChatOptions) (<-chan Chunk, error)
}

// NewClient 根据供应商配置创建对应的 LLM 客户端（自动检测协议）
func NewClient(providerType string, cfg ProviderConfig) (Client, error) {
	switch detectProvider(providerType, cfg.BaseURL) {
	case "claude":
		return NewClaudeClient(cfg), nil
	default:
		return NewOpenAIClient(cfg), nil
	}
}

// NewClientByProtocol 根据明确指定的协议字段创建 LLM 客户端
// protocol: "anthropic" / "openai-response" / "gemini" / "openai"(默认)
func NewClientByProtocol(protocol string, cfg ProviderConfig) (Client, error) {
	switch protocol {
	case "anthropic":
		return NewClaudeClient(cfg), nil
	case "openai-response":
		return NewOpenAIResponsesClient(cfg), nil
	case "gemini":
		return NewGeminiClient(cfg), nil
	default:
		return NewOpenAIClient(cfg), nil
	}
}

// detectProvider 根据供应商名称或 BaseURL 推断协议类型
func detectProvider(name, baseURL string) string {
	lower := strings.ToLower(name)
	lowerURL := strings.ToLower(baseURL)

	if strings.Contains(lower, "claude") || strings.Contains(lower, "anthropic") ||
		strings.Contains(lowerURL, "anthropic.com") {
		return "claude"
	}

	if strings.Contains(lower, "gemini") || strings.Contains(lower, "google") ||
		strings.Contains(lowerURL, "generativelanguage.googleapis.com") {
		return "gemini"
	}

	return "openai"
}

// QuickTest 使用最简消息测试连通性
func QuickTest(ctx context.Context, client Client, model string) (string, error) {
	resp, err := client.Chat(ctx, []Message{
		{Role: "user", Content: "Hi, respond with 'ok' only."},
	}, ChatOptions{
		Model:     model,
		MaxTokens: 10,
	})
	if err != nil {
		return "", fmt.Errorf("连通性测试失败: %w", err)
	}
	return resp.Content, nil
}
