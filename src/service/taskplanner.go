package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"agent_flow/src/provider"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// TaskPlanner 任务规划器，调用 LLM 生成阶段化任务计划
type TaskPlanner struct {
	TaskService  *TaskService
	ModelService *LLMModelService
	MainDB       *gorm.DB // 主 DB，用于查询 Agent 配置（Agent 表在主 DB）
}

// NewTaskPlanner 创建任务规划器
func NewTaskPlanner(taskSvc *TaskService, modelSvc *LLMModelService, mainDB *gorm.DB) *TaskPlanner {
	return &TaskPlanner{
		TaskService:  taskSvc,
		ModelService: modelSvc,
		MainDB:       mainDB,
	}
}

// planTask LLM 返回的单个任务结构
type planTask struct {
	Phase          int    `json:"phase"`
	PhaseLabel     string `json:"phase_label"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Priority       int    `json:"priority"`
	RequiredSkills string `json:"required_skills"` // 所需技能关键词
}

// PlanFromRequirement 从需求描述生成阶段化任务计划
// agentID=0 时使用全局默认模型（团队编排场景），否则使用指定 Agent 的模型
func (p *TaskPlanner) PlanFromRequirement(ctx context.Context, workspaceUUID string, workspaceID uint, agentID uint, requirement string) ([]*model.Task, error) {
	// 获取 LLM 客户端
	client, apiModelID, err := p.getClient(agentID)
	if err != nil {
		return nil, err
	}

	// 构建规划提示词
	prompt := fmt.Sprintf(`你是一个任务规划专家。请根据以下需求，生成分阶段的任务计划。

需求描述：
%s

请以 JSON 数组格式返回任务列表，每个任务包含以下字段：
- phase: 阶段编号（从1开始）
- phase_label: 阶段名称（如"需求分析"、"架构设计"、"编码实现"等）
- title: 任务标题（简洁明确）
- description: 任务描述（具体可执行的说明）
- priority: 优先级（0=普通 1=高 2=紧急）
- required_skills: 执行此任务所需的技能关键词（逗号分隔，如"PHP,MySQL"或"CSS,响应式布局"）

注意：
1. 阶段划分原则：每个阶段应是一个相对独立的工作单元，阶段内任务紧密相关，阶段间有明确的交付边界。系统会在每个阶段的所有任务全部完成后自动整理对话上下文（压缩历史、生成摘要、创建接力会话），因此：
   - 同一阶段内的任务应共享上下文依赖（如都需要参考同一份设计方案、操作同一层技术栈）
   - 跨阶段的信息传递应通过任务描述和工作文档完成，不应依赖对话记忆的延续
   - 每个阶段应有明确的交付物（如"后端 API 全部可用"、"核心页面开发完成"）
   - 典型划分方式：按技术栈分层（后端→前端→测试→文档）或按功能模块分批（模块A全栈→模块B全栈→集成测试）
2. 每阶段包含 2-5 个任务，过少会导致频繁整理增加开销，过多会导致上下文积累过大
3. 任务之间有明确的前后依赖关系，同阶段内的任务可并行，跨阶段任务有序推进
4. required_skills 用于智能分配任务给擅长该技能的团队成员
5. 只返回 JSON 数组，不要其他内容

返回格式示例：
[{"phase":1,"phase_label":"需求分析","title":"分析功能需求","description":"...","priority":0,"required_skills":"需求分析,文档"}]`, requirement)

	resp, err := client.Chat(ctx, []provider.Message{
		{Role: "user", Content: prompt},
	}, provider.ChatOptions{
		Model:     apiModelID,
		MaxTokens: 4096,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM 规划失败: %w", err)
	}

	// 解析 LLM 返回的 JSON
	content := extractJSON(resp.Content)
	var planned []planTask
	if err := json.Unmarshal([]byte(content), &planned); err != nil {
		slog.Error("解析任务计划失败", "content", resp.Content, "error", err)
		return nil, fmt.Errorf("解析任务计划失败: %w", err)
	}

	// 转换为 Task 模型
	var tasks []*model.Task
	for i, pt := range planned {
		task := &model.Task{
			WorkspaceID:    workspaceID,
			Title:          pt.Title,
			Description:    pt.Description,
			Phase:          pt.Phase,
			PhaseLabel:     pt.PhaseLabel,
			Priority:       pt.Priority,
			RequiredSkills: pt.RequiredSkills,
			Status:         0,
			Sort:           i,
		}
		tasks = append(tasks, task)
	}

	// 批量保存到工作区 DB
	if err := p.TaskService.BatchCreate(workspaceUUID, tasks); err != nil {
		return nil, fmt.Errorf("保存任务计划失败: %w", err)
	}

	return tasks, nil
}

// getClient 获取 LLM 客户端
// agentID=0 时使用全局默认模型，否则使用指定 Agent 的模型
func (p *TaskPlanner) getClient(agentID uint) (provider.Client, string, error) {
	if agentID == 0 {
		return p.getDefaultClient()
	}

	var agent model.Agent
	if err := p.MainDB.First(&agent, agentID).Error; err != nil {
		return nil, "", fmt.Errorf("AI 角色不存在: %w", err)
	}

	if agent.ModelID == "" {
		return nil, "", fmt.Errorf("AI 角色未配置模型")
	}

	m, err := p.ModelService.ResolveModel(agent.ModelID)
	if err != nil {
		return nil, "", err
	}

	client, err := p.ModelService.CreateClientFromModel(m)
	if err != nil {
		return nil, "", err
	}

	return client, m.APIModelID, nil
}

// getDefaultClient 获取全局默认模型的客户端（不依赖特定 Agent）
func (p *TaskPlanner) getDefaultClient() (provider.Client, string, error) {
	m, err := p.ModelService.GetDefault()
	if err != nil {
		return nil, "", fmt.Errorf("未设置默认模型，请在模型管理中设置: %w", err)
	}

	client, err := p.ModelService.CreateClientFromModel(m)
	if err != nil {
		return nil, "", fmt.Errorf("创建默认 LLM 客户端失败: %w", err)
	}

	return client, m.APIModelID, nil
}

// extractJSON 从 LLM 响应中提取 JSON 数组（跳过前后的文本）
func extractJSON(content string) string {
	start := -1
	end := -1
	depth := 0

	for i, ch := range content {
		if ch == '[' {
			if start == -1 {
				start = i
			}
			depth++
		}
		if ch == ']' {
			depth--
			if depth == 0 && start != -1 {
				end = i + 1
				break
			}
		}
	}

	if start >= 0 && end > start {
		return content[start:end]
	}
	return content
}
