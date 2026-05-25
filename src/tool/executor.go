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

			result := t.Execute(ctx, tc.Arguments)
			content := result.Content
			if result.IsError {
				content = "[错误] " + content
			}

			results[idx] = provider.Message{
				Role:       "tool",
				Content:    content,
				ToolCallID: tc.ID,
			}
		}(i, call)
	}

	wg.Wait()
	return results
}
