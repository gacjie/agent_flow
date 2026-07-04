package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"agent_flow/src/model"
	"agent_flow/src/provider"
)

// VisionCache 图片解析结果缓存（内存 + 持久化到 JSON 文件）
type VisionCache struct {
	Items map[string]string `json:"items"`
	path  string
	dirty bool
}

// NewVisionCache 创建缓存，若文件存在则加载历史数据
func NewVisionCache(filePath string) *VisionCache {
	c := &VisionCache{
		Items: make(map[string]string),
		path:  filePath,
	}
	if filePath != "" {
		if data, err := os.ReadFile(filePath); err == nil {
			_ = json.Unmarshal(data, c)
			if c.Items == nil {
				c.Items = make(map[string]string)
			}
		}
	}
	return c
}

func (c *VisionCache) Get(key string) (string, bool) {
	if c == nil || key == "" {
		return "", false
	}
	v, ok := c.Items[key]
	return v, ok
}

func (c *VisionCache) Set(key, value string) {
	if c == nil || key == "" {
		return
	}
	c.Items[key] = value
	c.dirty = true
}

// Save 将缓存写入 JSON 文件（仅在有新增时写入）
func (c *VisionCache) Save() error {
	if c == nil || !c.dirty || c.path == "" {
		return nil
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	c.dirty = false
	return os.WriteFile(c.path, data, 0644)
}

func sha256Hex(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

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
func (s *VisionService) ResolveImages(ctx context.Context, messages []provider.Message, m *model.LLMModel, cache *VisionCache) []provider.Message {
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
				// 缓存 key：优先用文件路径，无路径时用数据 hash
				cacheKey := part.Path
				if cacheKey == "" {
					cacheKey = sha256Hex(part.Data)
				}
				if cached, ok := cache.Get(cacheKey); ok {
					newParts = append(newParts, provider.ContentPart{Type: "text", Text: cached})
					continue
				}

				desc, err := s.DescribeImage(ctx, part.MediaType, part.Data)
				if err != nil {
					slog.Warn("视觉模型解析图片失败", "error", err)
					newParts = append(newParts, provider.ContentPart{
						Type: "text",
						Text: fmt.Sprintf("[图片解析失败: %s]", err.Error()),
					})
				} else {
					text := fmt.Sprintf("[图片描述: %s]", desc)
					cache.Set(cacheKey, text)
					newParts = append(newParts, provider.ContentPart{
						Type: "text",
						Text: text,
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
