package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"agent_flow/src/model"
)

// WorkflowServicer 工作流服务接口（避免 tool→service 循环依赖）
type WorkflowServicer interface {
	GetState(workspaceUUID string, conversationID uint) *model.WorkflowState
	GetStepPrompt(agentName string, stepNum int) string
	GetTotalSteps(agentName string) int
	GetStepLabel(agentName string, stepNum int) string
	GetIntro(agentName string) string
	AdvanceStepByUUID(workspaceUUID string, state *model.WorkflowState, step string, summary string, contextForNext string) error
	SaveStateByUUID(workspaceUUID string, state *model.WorkflowState) error
	RequestTruncate(workspaceUUID string, state *model.WorkflowState) error
	FormatStepHistory(state *model.WorkflowState, agentName string) string
}

// WorkFlowTool 工作流控制工具
type WorkFlowTool struct {
	WorkflowService WorkflowServicer
}

func (t *WorkFlowTool) Name() string { return "work_flow" }

func (t *WorkFlowTool) Description() string {
	return "工作流步骤管理。不传参数查询当前状态和流程列表；传 step 参数进入指定步骤或标记完成。空闲态时可用 step 参数直接进入任意步骤。"
}

func (t *WorkFlowTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"step": {
				"type": "string",
				"description": "目标步骤编号或 'done'。传编号（如 '3'）跳转到该步骤；传 'done' 标记工作流完成"
			},
			"summary": {
				"type": "string",
				"description": "当前步骤总结：干了什么、改了哪些文件、关键决策"
			},
			"context_for_next": {
				"type": "string",
				"description": "传递给下一步骤的内容，如规划步骤传执行计划，检查步骤传问题列表"
			},
			"truncate": {
				"type": "boolean",
				"description": "是否截断会话（触发上下文整理接力），截断后新会话从下一步骤开始"
			}
		}
	}`)
}

func (t *WorkFlowTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Step           string `json:"step"`
		Summary        string `json:"summary"`
		ContextForNext string `json:"context_for_next"`
		Truncate       bool   `json:"truncate"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	wc := GetWorkContext(ctx)
	if wc == nil || wc.WorkspaceUUID == "" {
		return ErrorResult("工作区未设置")
	}
	ai := GetAgentInfo(ctx)
	if ai == nil || ai.Name == "" {
		return ErrorResult("智能体信息未设置")
	}

	state := t.WorkflowService.GetState(wc.WorkspaceUUID, wc.ConversationID)
	if state == nil {
		return ErrorResult("当前会话未启用工作流")
	}

	// 无参数：查询模式
	if params.Step == "" && params.Summary == "" && params.ContextForNext == "" {
		return t.queryMode(state, ai.Name)
	}

	// 有参数：推进模式
	return t.advanceMode(ctx, wc, ai, state, params.Step, params.Summary, params.ContextForNext, params.Truncate)
}

func (t *WorkFlowTool) queryMode(state *model.WorkflowState, agentName string) *Result {
	var sb strings.Builder

	// 空闲态：返回概览
	if state.CurrentStepNum == 0 || state.Status == 0 {
		sb.WriteString("## 工作流状态: 空闲\n\n")

		intro := t.WorkflowService.GetIntro(agentName)
		if intro != "" {
			sb.WriteString(intro)
			sb.WriteString("\n\n")
		}

		sb.WriteString("### 可用阶段\n\n")
		totalSteps := t.WorkflowService.GetTotalSteps(agentName)
		for i := 1; i <= totalSteps; i++ {
			label := t.WorkflowService.GetStepLabel(agentName, i)
			sb.WriteString(fmt.Sprintf("  [STEP %d/%d] %s\n", i, totalSteps, label))
		}
		sb.WriteString("\n调用 work_flow(step=\"N\") 进入指定步骤。\n")

		// 历史记录
		historyStr := t.WorkflowService.FormatStepHistory(state, agentName)
		if historyStr != "" && historyStr != "(无工作流定义)" {
			sb.WriteString("\n### 流程执行记录\n\n")
			sb.WriteString(historyStr)
		}

		return SuccessResult(sb.String())
	}

	sb.WriteString(fmt.Sprintf("## 工作流状态\n\n"))
	sb.WriteString(fmt.Sprintf("当前步骤: [STEP %d/%d] %s\n\n",
		state.CurrentStepNum, state.TotalSteps,
		t.WorkflowService.GetStepLabel(agentName, state.CurrentStepNum)))

	// 当前步骤指引
	prompt := t.WorkflowService.GetStepPrompt(agentName, state.CurrentStepNum)
	if prompt != "" {
		sb.WriteString("### 步骤指引\n\n")
		sb.WriteString(prompt)
		sb.WriteString("\n\n")
	}

	// 上一步传递的内容
	if state.LastContext != "" {
		sb.WriteString("### 上一步传递内容\n\n")
		sb.WriteString(state.LastContext)
		sb.WriteString("\n\n")
	}

	// 流程列表
	sb.WriteString("### 流程执行记录\n\n")
	sb.WriteString(t.WorkflowService.FormatStepHistory(state, agentName))

	return SuccessResult(sb.String())
}

func (t *WorkFlowTool) advanceMode(ctx context.Context, wc *WorkContext, ai *AgentInfo, state *model.WorkflowState, step, summary, contextForNext string, truncate bool) *Result {
	if step == "" {
		return ErrorResult("推进模式需要提供 step 参数（步骤编号或 'done'）")
	}

	if err := t.WorkflowService.AdvanceStepByUUID(wc.WorkspaceUUID, state, step, summary, contextForNext); err != nil {
		return ErrorResult("步骤推进失败: " + err.Error())
	}

	// 截断请求
	if truncate {
		if err := t.WorkflowService.RequestTruncate(wc.WorkspaceUUID, state); err != nil {
			return ErrorResult("截断请求失败: " + err.Error())
		}
		return SuccessResult(fmt.Sprintf("步骤已完成，会话即将截断。下一步: [STEP %d/%d] %s",
			state.CurrentStepNum, state.TotalSteps,
			t.WorkflowService.GetStepLabel(ai.Name, state.CurrentStepNum)))
	}

	// 非截断：返回下一步的指引
	if state.Status == 2 {
		if state.RoundNum > 1 {
			return SuccessResult(fmt.Sprintf("轮次 %d 工作流已完成。", state.RoundNum))
		}
		return SuccessResult("工作流已完成，所有步骤执行结束。")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("步骤已推进到: [STEP %d/%d] %s\n\n",
		state.CurrentStepNum, state.TotalSteps,
		t.WorkflowService.GetStepLabel(ai.Name, state.CurrentStepNum)))

	prompt := t.WorkflowService.GetStepPrompt(ai.Name, state.CurrentStepNum)
	if prompt != "" {
		sb.WriteString("### 步骤指引\n\n")
		sb.WriteString(prompt)
		sb.WriteString("\n\n")
	}

	if state.LastContext != "" {
		sb.WriteString("### 上一步传递内容\n\n")
		sb.WriteString(state.LastContext)
	}

	return SuccessResult(sb.String())
}

// RegisterWorkflowTool 注册工作流工具
func RegisterWorkflowTool(r *Registry, svc WorkflowServicer) {
	r.Register(&WorkFlowTool{WorkflowService: svc})
}
