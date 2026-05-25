package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"agent_flow/src/provider"

	"gorm.io/gorm"
)

// LLMKeywordExtractor 基于 LLM 的关键词提取器
type LLMKeywordExtractor struct {
	DB            *gorm.DB
	ModelService  *LLMModelService
	PromptService *PromptService
}

// NewLLMKeywordExtractor 创建 LLM 关键词提取器
func NewLLMKeywordExtractor(db *gorm.DB, modelService *LLMModelService) *LLMKeywordExtractor {
	return &LLMKeywordExtractor{
		DB:           db,
		ModelService: modelService,
	}
}

// Extract 从文件内容中提取关键词和摘要
func (e *LLMKeywordExtractor) Extract(filePath, language, content string) (*KeywordResult, error) {
	// 获取默认模型
	m, err := e.ModelService.GetDefault()
	if err != nil {
		return nil, fmt.Errorf("未设置默认模型，请在模型管理中设置: %w", err)
	}

	client, err := e.ModelService.CreateClientFromModel(m)
	if err != nil {
		return nil, fmt.Errorf("创建 LLM 客户端失败: %w", err)
	}

	// 构建提示词
	tmpl := e.PromptService.GetPrompt("keyword_extract")
	prompt := fmt.Sprintf(tmpl, language, filePath, content)

	messages := []provider.Message{
		{Role: "user", Content: prompt},
	}

	opts := provider.ChatOptions{
		Model:     m.APIModelID,
		MaxTokens: 500,
	}

	resp, err := client.Chat(context.Background(), messages, opts)
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}

	// 解析 JSON 响应
	result, err := parseKeywordJSON(resp.Content)
	if err != nil {
		slog.Warn("关键词 JSON 解析失败，尝试提取", "response", resp.Content, "error", err)
		// 回退：直接使用响应作为摘要
		return &KeywordResult{
			Keywords: "",
			Summary:  truncateStr(resp.Content, 200),
		}, nil
	}

	return result, nil
}

// parseKeywordJSON 从 LLM 响应中解析关键词 JSON
func parseKeywordJSON(content string) (*KeywordResult, error) {
	content = strings.TrimSpace(content)

	// 尝试直接解析
	var result KeywordResult
	if err := json.Unmarshal([]byte(content), &result); err == nil {
		return &result, nil
	}

	// 尝试从 markdown 代码块中提取
	if idx := strings.Index(content, "{"); idx >= 0 {
		if end := strings.LastIndex(content, "}"); end > idx {
			jsonStr := content[idx : end+1]
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
				return &result, nil
			}
		}
	}

	return nil, fmt.Errorf("无法解析 JSON")
}

// truncateStr 截断字符串
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}
