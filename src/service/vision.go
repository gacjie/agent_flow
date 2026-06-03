package service

import (
	"context"
	"fmt"
	"log/slog"

	"agent_flow/src/model"
	"agent_flow/src/provider"
)

// VisionService 视觉代理服务（图片解析三级降级）
type VisionService struct {
	ModelService  *LLMModelService
	ConfigService *SysConfigService
	PromptService *PromptService
}

// NewVisionService 创建视觉代理服务
func NewVisionService(modelSvc *LLMModelService, configSvc *SysConfigService) *VisionService {
	return &VisionService{
		ModelService:  modelSvc,
		ConfigService: configSvc,
	}
}

// DescribeImage 调用第三方视觉模型解析图片，返回文本描述
func (s *VisionService) DescribeImage(ctx context.Context, mediaType, base64Data string) (string, error) {
	modelID := s.ConfigService.Get("ai.vision_model")
	if modelID == "" {
		return "", fmt.Errorf("未配置视觉解析模型")
	}

	m, err := s.ModelService.GetByModelID(modelID)
	if err != nil {
		return "", fmt.Errorf("视觉模型不存在: %s", modelID)
	}

	client, err := s.ModelService.CreateClientFromModel(m)
	if err != nil {
		return "", fmt.Errorf("创建视觉模型客户端失败: %w", err)
	}

	promptText := s.PromptService.GetPrompt("vision_parse")
	messages := []provider.Message{
		{
			Role: "user",
			ContentParts: []provider.ContentPart{
				{Type: "image", MediaType: mediaType, Data: base64Data},
				{Type: "text", Text: promptText},
			},
		},
	}

	resp, err := client.Chat(ctx, messages, provider.ChatOptions{
		Model:     m.APIModelID,
		MaxTokens: 1024,
	})
	if err != nil {
		return "", fmt.Errorf("视觉模型调用失败: %w", err)
	}

	return resp.Content, nil
}

// ResolveImages 处理消息中的图片（三级降级逻辑）
// 1. 模型支持视觉 → 保留图片 ContentParts 不变
// 2. 模型不支持视觉 + 已配置第三方 → 调用第三方解析为文本
// 3. 模型不支持视觉 + 未配置第三方 → 替换为提示文本
func (s *VisionService) ResolveImages(ctx context.Context, messages []provider.Message, m *model.LLMModel) []provider.Message {
	if m.HasCapability("vision") {
		return messages
	}

	visionModelID := s.ConfigService.Get("ai.vision_model")
	hasVisionModel := visionModelID != ""

	resolved := make([]provider.Message, len(messages))
	for i, msg := range messages {
		if len(msg.ContentParts) == 0 {
			resolved[i] = msg
			continue
		}

		var newParts []provider.ContentPart
		for _, part := range msg.ContentParts {
			if part.Type != "image" {
				newParts = append(newParts, part)
				continue
			}

			if hasVisionModel {
				desc, err := s.DescribeImage(ctx, part.MediaType, part.Data)
				if err != nil {
					slog.Warn("视觉模型解析图片失败", "error", err)
					newParts = append(newParts, provider.ContentPart{
						Type: "text",
						Text: fmt.Sprintf("[图片解析失败: %s]", err.Error()),
					})
				} else {
					newParts = append(newParts, provider.ContentPart{
						Type: "text",
						Text: fmt.Sprintf("[图片描述: %s]", desc),
					})
				}
			} else {
				newParts = append(newParts, provider.ContentPart{
					Type: "text",
					Text: "[图片无法解析：当前模型不支持视觉，且未配置视觉解析模型]",
				})
			}
		}

		resolved[i] = msg
		resolved[i].ContentParts = newParts

		// 如果所有 parts 都是 text，合并回 Content 字符串（优化：避免不必要的多模态格式）
		allText := true
		for _, p := range newParts {
			if p.Type != "text" {
				allText = false
				break
			}
		}
		if allText && len(newParts) > 0 {
			var combined string
			for _, p := range newParts {
				if combined != "" {
					combined += "\n"
				}
				combined += p.Text
			}
			resolved[i].Content = combined
			resolved[i].ContentParts = nil
		}
	}

	return resolved
}
