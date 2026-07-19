package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// UpdateMemoryTool AI 自主写入记忆工具（old_memory/new_memory 差异模式）
type UpdateMemoryTool struct{}

func (t *UpdateMemoryTool) Name() string { return "update_memories" }

func (t *UpdateMemoryTool) Description() string {
	return "更新当前智能体的记忆文件（memory.md），使用 old_memory/new_memory 差异模式。\n操作语义：只传 new_memory = 追加；同时传 old_memory 和 new_memory = 修改（精确匹配替换）；只传 old_memory = 删除。\n记忆文件属于智能体自身，跨工作区持久生效。"
}

func (t *UpdateMemoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"updates": {
				"type": "array",
				"description": "记忆更新列表，支持追加/修改/删除操作",
				"items": {
					"type": "object",
					"properties": {
						"old_memory": {
							"type": "string",
							"description": "要查找的旧记忆内容（修改/删除时必填，必须精确匹配记忆文件中的内容。追加时留空或不传）"
						},
						"new_memory": {
							"type": "string",
							"description": "新记忆内容（追加/修改时必填。删除时留空或不传）"
						}
					}
				}
			}
		},
		"required": ["updates"]
	}`)
}

func (t *UpdateMemoryTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Updates []struct {
			OldMemory string `json:"old_memory"`
			NewMemory string `json:"new_memory"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if len(params.Updates) == 0 {
		return ErrorResult("updates 参数不能为空（至少提供一项）")
	}
	for i, u := range params.Updates {
		if strings.TrimSpace(u.OldMemory) == "" && strings.TrimSpace(u.NewMemory) == "" {
			return ErrorResult(fmt.Sprintf("updates[%d] 的 old_memory 和 new_memory 不能同时为空", i))
		}
	}

	ai := GetAgentInfo(ctx)
	if ai == nil || ai.ContextRoot == "" || ai.Name == "" {
		return ErrorResult("智能体信息未设置（需要 AgentInfo）")
	}

	memoryPath := GetMemoryPath(ai.ContextRoot, ai.Name)

	var items []MemoryUpdateItem
	for _, u := range params.Updates {
		items = append(items, MemoryUpdateItem{
			OldMemory: u.OldMemory,
			NewMemory: u.NewMemory,
		})
	}

	results, hasError := ApplyMemoryUpdates(memoryPath, items)
	output := fmt.Sprintf("记忆更新结果（共 %d 项）：\n%s", len(items), strings.Join(results, "\n"))

	if hasError {
		return &Result{Content: output, IsError: false}
	}
	return SuccessResult(output)
}
