package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"agent_flow/src/config"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// WorkflowDef 解析后的工作流定义
type WorkflowDef struct {
	Intro string         // 工作流介绍（来自 ## 工作流介绍 段落）
	Steps []WorkflowStep
}

// WorkflowStep 单个步骤定义
type WorkflowStep struct {
	Num   int    // 步骤编号
	Total int    // 步骤总数
	Label string // 步骤标题
	Prompt string // 步骤内容（含完成条件和结束动作）
}

// WorkflowEngine 工作流引擎
type WorkflowEngine struct {
	WorkingDBMgr *WorkingDBManager
}

// NewWorkflowEngine 创建工作流引擎
func NewWorkflowEngine(dbMgr *WorkingDBManager) *WorkflowEngine {
	return &WorkflowEngine{WorkingDBMgr: dbMgr}
}

var stepHeaderRe = regexp.MustCompile(`(?m)^##\s*\[STEP\s+(\d+)/(\d+)\]\s*(.+)$`)
var introHeaderRe = regexp.MustCompile(`(?m)^##\s*工作流介绍\s*$`)

// ParseWorkflow 从 workflow.md 内容解析步骤定义
func (e *WorkflowEngine) ParseWorkflow(content string) *WorkflowDef {
	matches := stepHeaderRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	def := &WorkflowDef{}

	// 提取 ## 工作流介绍 段落
	if introLoc := introHeaderRe.FindStringIndex(content); introLoc != nil {
		introBodyStart := introLoc[1]
		// 介绍内容到下一个 ## 标题或第一个 STEP 之前
		introEnd := matches[0][0]
		// 检查是否有其他 ## 标题在介绍和第一个 STEP 之间
		otherH2 := regexp.MustCompile(`(?m)^##\s+`)
		if sub := content[introBodyStart:introEnd]; sub != "" {
			if loc := otherH2.FindStringIndex(sub); loc != nil {
				introEnd = introBodyStart + loc[0]
			}
		}
		def.Intro = strings.TrimSpace(content[introBodyStart:introEnd])
	}

	for i, match := range matches {
		numStr := content[match[2]:match[3]]
		totalStr := content[match[4]:match[5]]
		label := strings.TrimSpace(content[match[6]:match[7]])

		num := 0
		total := 0
		fmt.Sscanf(numStr, "%d", &num)
		fmt.Sscanf(totalStr, "%d", &total)

		// 步骤内容是当前标题之后到下一个标题之前
		bodyStart := match[1]
		var bodyEnd int
		if i+1 < len(matches) {
			bodyEnd = matches[i+1][0]
		} else {
			bodyEnd = len(content)
		}
		prompt := strings.TrimSpace(content[bodyStart:bodyEnd])

		def.Steps = append(def.Steps, WorkflowStep{
			Num:   num,
			Total: total,
			Label: label,
			Prompt: prompt,
		})
	}
	return def
}

// LoadDefinition 加载指定智能体的工作流定义
func (e *WorkflowEngine) LoadDefinition(agentName string) *WorkflowDef {
	if agentName == "" {
		return nil
	}
	contextRoot := config.Get().Agent.ContextRoot
	workflowPath := filepath.Join(contextRoot, "agents", agentName, "workflow.md")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return nil
	}
	return e.ParseWorkflow(string(data))
}

// GetOrCreateState 获取或创建会话的工作流状态
func (e *WorkflowEngine) GetOrCreateState(db *gorm.DB, conversationID uint, agentName string, def *WorkflowDef) *model.WorkflowState {
	var state model.WorkflowState
	if err := db.Where("conversation_id = ?", conversationID).First(&state).Error; err == nil {
		return &state
	}

	state = model.WorkflowState{
		ConversationID: conversationID,
		AgentName:      agentName,
		CurrentStepNum: 0,
		TotalSteps:     len(def.Steps),
		Status:         0,
		RoundNum:       1,
	}
	db.Create(&state)
	return &state
}

// SaveState 保存工作流状态
func (e *WorkflowEngine) SaveState(db *gorm.DB, state *model.WorkflowState) {
	db.Save(state)
}

// StartNewRound 开启新轮次（工作流完成后用户再发消息时调用）
func (e *WorkflowEngine) StartNewRound(db *gorm.DB, state *model.WorkflowState, def *WorkflowDef) {
	state.RoundNum++
	state.CurrentStepNum = 0
	state.Status = 0
	state.Corrections = 0
	state.LastContext = ""
	e.SaveState(db, state)
}

// GetStepByNum 根据编号获取步骤定义
func (e *WorkflowEngine) GetStepByNum(def *WorkflowDef, num int) *WorkflowStep {
	for i := range def.Steps {
		if def.Steps[i].Num == num {
			return &def.Steps[i]
		}
	}
	return nil
}

// BuildStepContext 构建当前步骤的注入内容
func (e *WorkflowEngine) BuildStepContext(state *model.WorkflowState, def *WorkflowDef) string {
	step := e.GetStepByNum(def, state.CurrentStepNum)
	if step == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<workflow_status>\n")
	if state.RoundNum > 1 {
		sb.WriteString(fmt.Sprintf("轮次: %d\n", state.RoundNum))
	}
	sb.WriteString(fmt.Sprintf("当前步骤: [STEP %d/%d] %s\n\n", step.Num, step.Total, step.Label))
	sb.WriteString("步骤指引:\n")
	sb.WriteString(step.Prompt)
	sb.WriteString("\n")

	if state.LastContext != "" {
		sb.WriteString("\n上一步传递内容:\n")
		sb.WriteString(state.LastContext)
		sb.WriteString("\n")
	}

	sb.WriteString("\n提醒: 完成当前步骤后请调用 work_flow 工具推进到下一步。")
	sb.WriteString("\n</workflow_status>")
	return sb.String()
}

// BuildOverviewContext 构建空闲态的工作流概览注入内容
func (e *WorkflowEngine) BuildOverviewContext(state *model.WorkflowState, def *WorkflowDef) string {
	var sb strings.Builder
	sb.WriteString("<workflow_overview>\n")

	if def.Intro != "" {
		sb.WriteString(def.Intro)
		sb.WriteString("\n\n")
	}

	sb.WriteString("可用阶段:\n")
	for _, s := range def.Steps {
		sb.WriteString(fmt.Sprintf("  [STEP %d/%d] %s\n", s.Num, s.Total, s.Label))
	}

	sb.WriteString("\n判断用户请求是否匹配本工作流处理范围:\n")
	sb.WriteString("- 匹配时调用 work_flow(step=\"1\") 从头开始，或 work_flow(step=\"N\") 进入指定步骤\n")
	sb.WriteString("- 不匹配时直接回复用户，无需进入工作流\n")

	if state.RoundNum > 1 {
		sb.WriteString(fmt.Sprintf("\n当前轮次: %d\n", state.RoundNum))
	}
	if state.LastContext != "" {
		sb.WriteString("\n上一轮传递内容:\n")
		sb.WriteString(state.LastContext)
		sb.WriteString("\n")
	}

	sb.WriteString("</workflow_overview>")
	return sb.String()
}

// BuildCorrectionMessage 构建纠正消息
func (e *WorkflowEngine) BuildCorrectionMessage(state *model.WorkflowState, def *WorkflowDef) string {
	step := e.GetStepByNum(def, state.CurrentStepNum)
	if step == nil {
		return "请调用 work_flow 工具推进工作流。"
	}
	return fmt.Sprintf(
		"[工作流引导] 当前正在执行步骤 [STEP %d/%d] %s，该步骤尚未完成。请继续按步骤要求操作，完成后调用 work_flow 工具推进到下一步。",
		step.Num, step.Total, step.Label)
}

// AdvanceStep 推进到指定步骤
func (e *WorkflowEngine) AdvanceStep(db *gorm.DB, state *model.WorkflowState, def *WorkflowDef, targetStep int, summary string, contextForNext string) {
	if state.Status == 0 {
		// 从空闲态进入：不记录历史，直接激活目标步骤
		state.CurrentStepNum = targetStep
		state.Status = 1
		state.Corrections = 0
		state.LastContext = contextForNext
		e.SaveState(db, state)
		return
	}

	// 记录当前步骤到历史
	var history []model.StepHistoryEntry
	if state.StepHistory != "" {
		json.Unmarshal([]byte(state.StepHistory), &history)
	}
	currentStep := e.GetStepByNum(def, state.CurrentStepNum)
	label := ""
	if currentStep != nil {
		label = currentStep.Label
	}
	history = append(history, model.StepHistoryEntry{
		Round:          state.RoundNum,
		Step:           state.CurrentStepNum,
		Label:          label,
		Summary:        summary,
		ContextForNext: contextForNext,
	})
	historyJSON, _ := json.Marshal(history)
	state.StepHistory = string(historyJSON)
	state.LastContext = contextForNext
	state.CurrentStepNum = targetStep
	state.Corrections = 0

	e.SaveState(db, state)
}

// MarkDone 标记工作流完成
func (e *WorkflowEngine) MarkDone(db *gorm.DB, state *model.WorkflowState, def *WorkflowDef, summary string) {
	var history []model.StepHistoryEntry
	if state.StepHistory != "" {
		json.Unmarshal([]byte(state.StepHistory), &history)
	}
	currentStep := e.GetStepByNum(def, state.CurrentStepNum)
	label := ""
	if currentStep != nil {
		label = currentStep.Label
	}
	history = append(history, model.StepHistoryEntry{
		Round:   state.RoundNum,
		Step:    state.CurrentStepNum,
		Label:   label,
		Summary: summary,
	})
	historyJSON, _ := json.Marshal(history)
	state.StepHistory = string(historyJSON)
	state.Status = 2

	e.SaveState(db, state)
}

// ForceAdvance 强制推进到下一步（纠正超限时使用）
func (e *WorkflowEngine) ForceAdvance(db *gorm.DB, state *model.WorkflowState, def *WorkflowDef) {
	nextNum := e.findNextStepNum(def, state.CurrentStepNum)
	if nextNum == 0 {
		// 已是最后一步，标记完成
		e.MarkDone(db, state, def, "(强制推进)")
		return
	}
	e.AdvanceStep(db, state, def, nextNum, "(强制推进)", "")
}

// ForceAdvanceByUUID 强制推进（WorkflowServicer 场景，纠正超限时使用）
func (e *WorkflowEngine) ForceAdvanceByUUID(workspaceUUID string, state *model.WorkflowState) error {
	db, err := e.WorkingDBMgr.GetDB(workspaceUUID)
	if err != nil {
		return err
	}
	def := e.LoadDefinition(state.AgentName)
	if def == nil {
		return fmt.Errorf("无法加载工作流定义: %s", state.AgentName)
	}
	e.ForceAdvance(db, state, def)
	return nil
}

// findNextStepNum 找到下一个步骤编号
func (e *WorkflowEngine) findNextStepNum(def *WorkflowDef, currentNum int) int {
	for i, step := range def.Steps {
		if step.Num == currentNum && i+1 < len(def.Steps) {
			return def.Steps[i+1].Num
		}
	}
	return 0
}

// IsCompleted 工作流是否已完成
func (e *WorkflowEngine) IsCompleted(state *model.WorkflowState) bool {
	return state.Status == 2
}

// BuildQueryResponse 构建查询模式的输出
func (e *WorkflowEngine) BuildQueryResponse(state *model.WorkflowState, def *WorkflowDef) string {
	var sb strings.Builder

	// 当前步骤信息
	step := e.GetStepByNum(def, state.CurrentStepNum)
	if step != nil {
		if state.RoundNum > 1 {
			sb.WriteString(fmt.Sprintf("## 轮次 %d — 当前步骤: [STEP %d/%d] %s\n\n", state.RoundNum, step.Num, step.Total, step.Label))
		} else {
			sb.WriteString(fmt.Sprintf("## 当前步骤: [STEP %d/%d] %s\n\n", step.Num, step.Total, step.Label))
		}
		sb.WriteString(step.Prompt)
		sb.WriteString("\n")
	}

	// 上一步传递内容
	if state.LastContext != "" {
		sb.WriteString("\n## 上一步传递内容\n\n")
		sb.WriteString(state.LastContext)
		sb.WriteString("\n")
	}

	// 流程列表
	sb.WriteString("\n## 流程列表\n\n")
	sb.WriteString(e.FormatStepHistory(state, state.AgentName))

	return sb.String()
}

// HasWorkflow 检查指定智能体是否有有效工作流
func (e *WorkflowEngine) HasWorkflow(agentName string) bool {
	return e.LoadDefinition(agentName) != nil
}

// InheritState 继承旧会话的工作流状态到新会话（TidyUp 接力用）
func (e *WorkflowEngine) InheritState(db *gorm.DB, oldState *model.WorkflowState, newConvID uint) *model.WorkflowState {
	newState := model.WorkflowState{
		ConversationID: newConvID,
		AgentName:      oldState.AgentName,
		CurrentStepNum: oldState.CurrentStepNum,
		TotalSteps:     oldState.TotalSteps,
		Status:         oldState.Status,
		RoundNum:       oldState.RoundNum,
		StepHistory:    oldState.StepHistory,
		LastContext:    oldState.LastContext,
	}
	db.Create(&newState)
	return &newState
}

// CarryOverState 截断接力：将当前工作流状态继承到新会话，清除截断标志（WorkflowServicer 接口）
func (e *WorkflowEngine) CarryOverState(workspaceUUID string, oldState *model.WorkflowState, newConvID uint) *model.WorkflowState {
	db, err := e.WorkingDBMgr.GetDB(workspaceUUID)
	if err != nil {
		return nil
	}
	newState := model.WorkflowState{
		ConversationID: newConvID,
		AgentName:      oldState.AgentName,
		CurrentStepNum: oldState.CurrentStepNum,
		TotalSteps:     oldState.TotalSteps,
		Status:         oldState.Status,
		RoundNum:       oldState.RoundNum,
		StepHistory:    oldState.StepHistory,
		LastContext:    oldState.LastContext,
		// TruncateRequested 不继承（默认 false），Corrections 归零
	}
	if err := db.Create(&newState).Error; err != nil {
		return nil
	}
	return &newState
}

// GetStateByConversation 获取指定会话的工作流状态
func (e *WorkflowEngine) GetStateByConversation(db *gorm.DB, conversationID uint) *model.WorkflowState {
	var state model.WorkflowState
	if err := db.Where("conversation_id = ?", conversationID).First(&state).Error; err != nil {
		return nil
	}
	return &state
}

// ===== WorkflowServicer 接口实现（供 tool 包通过接口调用） =====

// GetState 获取会话的工作流状态（WorkflowServicer 接口）
func (e *WorkflowEngine) GetState(workspaceUUID string, conversationID uint) *model.WorkflowState {
	db, err := e.WorkingDBMgr.GetDB(workspaceUUID)
	if err != nil {
		return nil
	}
	return e.GetStateByConversation(db, conversationID)
}

// GetStepPrompt 获取指定步骤的 prompt 内容（WorkflowServicer 接口）
func (e *WorkflowEngine) GetStepPrompt(agentName string, stepNum int) string {
	def := e.LoadDefinition(agentName)
	if def == nil {
		return ""
	}
	step := e.GetStepByNum(def, stepNum)
	if step == nil {
		return ""
	}
	return step.Prompt
}

// GetIntro 获取工作流介绍内容（WorkflowServicer 接口）
func (e *WorkflowEngine) GetIntro(agentName string) string {
	def := e.LoadDefinition(agentName)
	if def == nil {
		return ""
	}
	return def.Intro
}

// GetTotalSteps 获取智能体工作流总步骤数（WorkflowServicer 接口）
func (e *WorkflowEngine) GetTotalSteps(agentName string) int {
	def := e.LoadDefinition(agentName)
	if def == nil {
		return 0
	}
	return len(def.Steps)
}

// GetStepLabel 获取指定步骤的标题（WorkflowServicer 接口）
func (e *WorkflowEngine) GetStepLabel(agentName string, stepNum int) string {
	def := e.LoadDefinition(agentName)
	if def == nil {
		return ""
	}
	step := e.GetStepByNum(def, stepNum)
	if step == nil {
		return ""
	}
	return step.Label
}

// AdvanceStep 推进步骤（WorkflowServicer 接口，供 tool 调用）
func (e *WorkflowEngine) AdvanceStepByUUID(workspaceUUID string, state *model.WorkflowState, step string, summary string, contextForNext string) error {
	db, err := e.WorkingDBMgr.GetDB(workspaceUUID)
	if err != nil {
		return fmt.Errorf("获取工作区数据库失败: %w", err)
	}
	def := e.LoadDefinition(state.AgentName)
	if def == nil {
		return fmt.Errorf("无法加载工作流定义: %s", state.AgentName)
	}

	if step == "done" {
		e.MarkDone(db, state, def, summary)
		return nil
	}

	var targetNum int
	if _, err := fmt.Sscanf(step, "%d", &targetNum); err != nil {
		return fmt.Errorf("无效的步骤编号: %s", step)
	}
	targetStep := e.GetStepByNum(def, targetNum)
	if targetStep == nil {
		return fmt.Errorf("步骤 %d 不存在", targetNum)
	}

	e.AdvanceStep(db, state, def, targetNum, summary, contextForNext)
	return nil
}

// SaveStateByUUID 保存状态（WorkflowServicer 接口）
func (e *WorkflowEngine) SaveStateByUUID(workspaceUUID string, state *model.WorkflowState) error {
	db, err := e.WorkingDBMgr.GetDB(workspaceUUID)
	if err != nil {
		return err
	}
	e.SaveState(db, state)
	return nil
}

// RequestTruncate 标记截断请求（WorkflowServicer 接口）
// 设置 state 的 TruncateRequested 标志，由 ChatRunner 在下一轮检测并执行
func (e *WorkflowEngine) RequestTruncate(workspaceUUID string, state *model.WorkflowState) error {
	state.TruncateRequested = true
	return e.SaveStateByUUID(workspaceUUID, state)
}

// FormatStepHistory 格式化步骤执行历史（WorkflowServicer 接口）
func (e *WorkflowEngine) FormatStepHistory(state *model.WorkflowState, agentName string) string {
	def := e.LoadDefinition(agentName)
	if def == nil {
		return "(无工作流定义)"
	}

	var history []model.StepHistoryEntry
	if state.StepHistory != "" {
		json.Unmarshal([]byte(state.StepHistory), &history)
	}

	// 确定最大轮次（兼容旧数据 Round==0 视为 1）
	maxRound := state.RoundNum
	if maxRound < 1 {
		maxRound = 1
	}

	// 按轮次分组历史条目
	roundHistory := make(map[int]map[int][]model.StepHistoryEntry) // round → step → entries
	for _, h := range history {
		r := h.Round
		if r == 0 {
			r = 1
		}
		if roundHistory[r] == nil {
			roundHistory[r] = make(map[int][]model.StepHistoryEntry)
		}
		roundHistory[r][h.Step] = append(roundHistory[r][h.Step], h)
	}

	var sb strings.Builder

	for round := 1; round <= maxRound; round++ {
		isCurrent := round == state.RoundNum && state.Status == 1
		isCompleted := round < state.RoundNum || (round == state.RoundNum && state.Status == 2)

		// 多轮次时显示轮次标题
		if maxRound > 1 {
			if isCompleted {
				sb.WriteString(fmt.Sprintf("### 轮次 %d ✓\n", round))
			} else if isCurrent {
				sb.WriteString(fmt.Sprintf("### 轮次 %d（当前）\n", round))
			} else {
				sb.WriteString(fmt.Sprintf("### 轮次 %d\n", round))
			}
		}

		stepMap := roundHistory[round]

		for _, s := range def.Steps {
			entries := stepMap[s.Num]
			if isCurrent && s.Num == state.CurrentStepNum {
				sb.WriteString(fmt.Sprintf("- [STEP %d] %s ← 当前\n", s.Num, s.Label))
			} else if len(entries) > 0 {
				last := entries[len(entries)-1]
				summaryStr := ""
				if last.Summary != "" {
					summaryStr = " — " + last.Summary
				}
				sb.WriteString(fmt.Sprintf("- [STEP %d] %s ✓%s\n", s.Num, s.Label, summaryStr))
			} else if isCurrent {
				sb.WriteString(fmt.Sprintf("- [STEP %d] %s\n", s.Num, s.Label))
			}
		}

		// 回退记录（同轮次内超过步骤数的条目表示有回退）
		var roundEntries []model.StepHistoryEntry
		for _, h := range history {
			r := h.Round
			if r == 0 {
				r = 1
			}
			if r == round {
				roundEntries = append(roundEntries, h)
			}
		}
		if len(roundEntries) > len(def.Steps) {
			sb.WriteString("  执行路径: ")
			for i, h := range roundEntries {
				if i > 0 {
					sb.WriteString(" → ")
				}
				sb.WriteString(fmt.Sprintf("%s(%d)", h.Label, h.Step))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
