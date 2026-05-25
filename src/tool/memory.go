package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// UpdateMemoryTool AI 自主写入记忆工具（支持单条和批量，old_memory/new_memory 差异模式）
type UpdateMemoryTool struct{}

func (t *UpdateMemoryTool) Name() string { return "update_memories" }

func (t *UpdateMemoryTool) Description() string {
	return "更新当前智能体的记忆文件（memory.md），使用 old_memory/new_memory 差异模式。支持单条和批量操作。\n操作语义：只传 new_memory = 追加；同时传 old_memory 和 new_memory = 修改（精确匹配替换）；只传 old_memory = 删除。\n记忆文件属于智能体自身，跨工作区持久生效。"
}

func (t *UpdateMemoryTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"old_memory": {
				"type": "string",
				"description": "（单条模式）要查找的旧记忆内容（修改/删除时必填，必须精确匹配记忆文件中的内容。追加时留空或不传）"
			},
			"new_memory": {
				"type": "string",
				"description": "（单条模式）新记忆内容（追加/修改时必填。删除时留空或不传）"
			},
			"updates": {
				"type": "array",
				"description": "（批量模式）记忆更新列表，存在且非空时优先使用。支持追加/修改/删除操作",
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
		}
	}`)
}

func (t *UpdateMemoryTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		OldMemory string `json:"old_memory"`
		NewMemory string `json:"new_memory"`
		Updates   []struct {
			OldMemory string `json:"old_memory"`
			NewMemory string `json:"new_memory"`
		} `json:"updates"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	ai := GetAgentInfo(ctx)
	if ai == nil || ai.ContextRoot == "" || ai.Name == "" {
		return ErrorResult("智能体信息未设置（需要 AgentInfo）")
	}

	memoryPath := GetMemoryPath(ai.ContextRoot, ai.Name)

	// 构建更新列表：批量模式优先
	var items []MemoryUpdateItem
	if len(params.Updates) > 0 {
		for _, u := range params.Updates {
			items = append(items, MemoryUpdateItem{
				OldMemory: u.OldMemory,
				NewMemory: u.NewMemory,
			})
		}
	} else if params.OldMemory != "" || params.NewMemory != "" {
		items = append(items, MemoryUpdateItem{
			OldMemory: params.OldMemory,
			NewMemory: params.NewMemory,
		})
	} else {
		return ErrorResult("记忆内容不能为空（需要 old_memory/new_memory 或 updates 数组）")
	}

	results, hasError := ApplyMemoryUpdates(memoryPath, items)
	output := fmt.Sprintf("记忆更新结果（共 %d 项）：\n%s", len(items), strings.Join(results, "\n"))

	if hasError {
		return &Result{Content: output, IsError: false}
	}
	return SuccessResult(output)
}
