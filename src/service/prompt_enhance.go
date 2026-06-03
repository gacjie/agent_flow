package service

import (
	"context"
	"errors"
	"fmt"

	"agent_flow/src/provider"
)

// ErrPromptTruncated 增强结果被截断（输出达到模型 token 上限）
var ErrPromptTruncated = errors.New("增强结果可能不完整（输出达到 token 上限）")

// PromptEnhanceService 提示词增强服务
type PromptEnhanceService struct {
	ModelService  *LLMModelService
	ConfigService *SysConfigService
	PromptService *PromptService
}

// NewPromptEnhanceService 创建提示词增强服务
func NewPromptEnhanceService(modelSvc *LLMModelService, configSvc *SysConfigService, promptSvc *PromptService) *PromptEnhanceService {
	return &PromptEnhanceService{
		ModelService:  modelSvc,
		ConfigService: configSvc,
		PromptService: promptSvc,
	}
}

// Enhance 增强用户输入的提示词
func (s *PromptEnhanceService) Enhance(ctx context.Context, userInput string) (string, error) {
	modelID := s.ConfigService.Get("ai.prompt_enhance_model")
	if modelID == "" {
		modelID = "auto"
	}

	m, err := s.ModelService.ResolveModel(modelID)
	if err != nil {
		return "", fmt.Errorf("获取增强模型失败: %w", err)
	}

	client, err := s.ModelService.CreateClientFromModel(m)
	if err != nil {
		return "", fmt.Errorf("创建客户端失败: %w", err)
	}

	systemPrompt := s.getEnhancePrompt()

	messages := []provider.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userInput},
	}

	maxTokens := m.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	resp, err := client.Chat(ctx, messages, provider.ChatOptions{
		Model:     m.APIModelID,
		MaxTokens: maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("模型调用失败: %w", err)
	}

	if resp.FinishReason == "length" || resp.FinishReason == "max_tokens" {
		return resp.Content, ErrPromptTruncated
	}

	return resp.Content, nil
}

func (s *PromptEnhanceService) getEnhancePrompt() string {
	if s.PromptService != nil {
		if tpl := s.PromptService.GetPrompt("prompt_enhance"); tpl != "" {
			return tpl
		}
	}
	return `你是一个提示词优化专家。用户会给你一段原始提示词，请你优化它使其更加清晰、具体、有效。

优化原则：
1. 保持用户的原始意图不变
2. 使表达更加清晰和具体
3. 补充必要的上下文和约束条件
4. 优化结构，使其更易于 AI 理解和执行
5. 如果原始提示词已经很好，只做微调

直接输出优化后的提示词，不要添加解释或前缀。`
}
