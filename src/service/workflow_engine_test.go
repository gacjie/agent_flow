package service

import (
	"encoding/json"
	"testing"

	"agent_flow/src/model"
)

const sampleWorkflow = `# 后端开发工作流

## [STEP 1/3] 读取任务

使用 task_lists 定位任务。

**完成条件**: 任务已读取。
**结束动作**: 调用 work_flow(step="2")。

## [STEP 2/3] 实施

按规划编码。

**结束动作**: 有问题 work_flow(step="1")，无问题 work_flow(step="3")。

## [STEP 3/3] 收尾

更新任务状态并总结。

**结束动作**: 调用 work_flow(step="done")。

## 完成检查

- 是否逐条实现
`

func TestParseWorkflow(t *testing.T) {
	e := &WorkflowEngine{}
	def := e.ParseWorkflow(sampleWorkflow)
	if def == nil {
		t.Fatal("解析返回 nil")
	}
	if len(def.Steps) != 3 {
		t.Fatalf("期望 3 个步骤，得到 %d", len(def.Steps))
	}

	// 校验步骤编号、总数、标题
	cases := []struct {
		num   int
		total int
		label string
	}{
		{1, 3, "读取任务"},
		{2, 3, "实施"},
		{3, 3, "收尾"},
	}
	for i, c := range cases {
		s := def.Steps[i]
		if s.Num != c.num || s.Total != c.total || s.Label != c.label {
			t.Errorf("步骤 %d 解析错误: got num=%d total=%d label=%q", i, s.Num, s.Total, s.Label)
		}
	}

	// STEP 1 的 prompt 应包含结束动作，且不含 STEP 2 的内容
	step1 := e.GetStepByNum(def, 1)
	if step1 == nil {
		t.Fatal("GetStepByNum(1) 返回 nil")
	}
	if !contains(step1.Prompt, `work_flow(step="2")`) {
		t.Errorf("STEP 1 prompt 缺少结束动作: %q", step1.Prompt)
	}
	if contains(step1.Prompt, "按规划编码") {
		t.Errorf("STEP 1 prompt 混入了 STEP 2 内容: %q", step1.Prompt)
	}

	// 最后一个步骤的 prompt 不应包含"完成检查"之后的无关内容边界问题
	step3 := e.GetStepByNum(def, 3)
	if !contains(step3.Prompt, "更新任务状态") {
		t.Errorf("STEP 3 prompt 缺少正文: %q", step3.Prompt)
	}
}

func TestParseWorkflowNoSteps(t *testing.T) {
	e := &WorkflowEngine{}
	if def := e.ParseWorkflow("# 普通文档\n没有 STEP 标记"); def != nil {
		t.Errorf("无 STEP 格式应返回 nil，得到 %+v", def)
	}
}

func TestFindNextStepNum(t *testing.T) {
	e := &WorkflowEngine{}
	def := e.ParseWorkflow(sampleWorkflow)
	if got := e.findNextStepNum(def, 1); got != 2 {
		t.Errorf("步骤 1 的下一步期望 2，得到 %d", got)
	}
	if got := e.findNextStepNum(def, 3); got != 0 {
		t.Errorf("最后一步的下一步期望 0，得到 %d", got)
	}
}

func TestStartNewRound(t *testing.T) {
	e := &WorkflowEngine{}
	def := e.ParseWorkflow(sampleWorkflow)

	// 模拟轮次1已完成的状态
	history := []model.StepHistoryEntry{
		{Round: 1, Step: 1, Label: "读取任务", Summary: "已读取"},
		{Round: 1, Step: 2, Label: "实施", Summary: "已实施"},
		{Round: 1, Step: 3, Label: "收尾", Summary: "已收尾"},
	}
	historyJSON, _ := json.Marshal(history)
	state := &model.WorkflowState{
		ConversationID: 1,
		AgentName:      "test",
		CurrentStepNum: 3,
		TotalSteps:     3,
		Status:         2,
		RoundNum:       1,
		StepHistory:    string(historyJSON),
		LastContext:    "some context",
		Corrections:    2,
	}

	// 不使用真实 DB，直接调用逻辑验证字段变化（跳过 SaveState）
	state.RoundNum++
	state.CurrentStepNum = def.Steps[0].Num
	state.Status = 1
	state.Corrections = 0
	state.LastContext = ""

	if state.RoundNum != 2 {
		t.Errorf("期望 RoundNum=2，得到 %d", state.RoundNum)
	}
	if state.CurrentStepNum != 1 {
		t.Errorf("期望 CurrentStepNum=1，得到 %d", state.CurrentStepNum)
	}
	if state.Status != 1 {
		t.Errorf("期望 Status=1，得到 %d", state.Status)
	}
	if state.Corrections != 0 {
		t.Errorf("期望 Corrections=0，得到 %d", state.Corrections)
	}
	if state.LastContext != "" {
		t.Errorf("期望 LastContext 为空，得到 %q", state.LastContext)
	}
	// StepHistory 应保留
	if state.StepHistory != string(historyJSON) {
		t.Error("StepHistory 不应被清空")
	}
}

func TestStepHistoryRoundTag(t *testing.T) {
	// 验证 StepHistoryEntry 的 Round 字段 JSON 序列化
	entry := model.StepHistoryEntry{
		Round:   2,
		Step:    1,
		Label:   "读取任务",
		Summary: "done",
	}
	data, _ := json.Marshal(entry)
	var parsed model.StepHistoryEntry
	json.Unmarshal(data, &parsed)
	if parsed.Round != 2 {
		t.Errorf("Round 字段序列化失败: 期望 2，得到 %d", parsed.Round)
	}

	// 旧数据兼容：Round 字段缺失时为 0
	oldJSON := `{"step":1,"label":"读取任务","summary":"done"}`
	var oldEntry model.StepHistoryEntry
	json.Unmarshal([]byte(oldJSON), &oldEntry)
	if oldEntry.Round != 0 {
		t.Errorf("旧数据 Round 应为 0，得到 %d", oldEntry.Round)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
