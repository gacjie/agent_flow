package tool

import (
	"context"
	"sync"

	"agent_flow/src/provider"
)

// Executor 工具并行执行器
type Executor struct {
	Registry *Registry
}

// NewExecutor 创建执行器
func NewExecutor(registry *Registry) *Executor {
	return &Executor{Registry: registry}
}

// Execute 并行执行多个工具调用，返回 tool 类型的消息列表
func (e *Executor) Execute(ctx context.Context, calls []provider.ToolCall) []provider.Message {
	results := make([]provider.Message, len(calls))
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, tc provider.ToolCall) {
			defer wg.Done()

			t, ok := e.Registry.Get(tc.Name)
			if !ok {
				results[idx] = provider.Message{
					Role:       "tool",
					Content:    "工具不存在: " + tc.Name,
					ToolCallID: tc.ID,
				}
				return
			}

			toolCtx := WithToolCallID(ctx, tc.ID)
			result := t.Execute(toolCtx, tc.Arguments)
			content := result.Content
			if result.IsError {
				content = "[错误] " + content
			}

			msg := provider.Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: tc.ID,
			}

			// 工具返回图片时，构建 ContentParts（由 ChatRunner 层统一处理视觉逻辑）
			if len(result.Images) > 0 {
				parts := []provider.ContentPart{{Type: "text", Text: content}}
				for _, img := range result.Images {
					parts = append(parts, provider.ContentPart{
						Type:      "image",
						MediaType: img.MediaType,
						Data:      img.Data,
						Path:      img.Path,
					})
				}
				msg.ContentParts = parts
			}

			results[idx] = msg
		}(i, call)
	}

	wg.Wait()
	return results
}
