package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent_flow/src/model"
	"agent_flow/src/tool/tasks"
)

// ---- task_lists ----

// TaskListsTool 列出工作区任务（树形结构，支持按阶段/状态过滤）
type TaskListsTool struct {
	TaskService TaskServicer
}

func (t *TaskListsTool) Name() string { return "task_lists" }

func (t *TaskListsTool) Description() string {
	return "查看当前工作区的任务列表（树形结构）。支持按阶段和状态过滤。指定 task_id 查询某个任务的基本信息（状态、标题、描述、结果、子任务概览）；加 verbose=true 时额外携带详细说明及关联任务文档内容。不传参数则展示所有任务的精简列表。"
}

func (t *TaskListsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task_id": {
				"type": "integer",
				"description": "查询指定任务信息及其子任务概览，默认返回基本信息；配合 verbose=true 获取完整上下文（含 detail 和关联任务文档内容）"
			},
			"phase": {
				"type": "integer",
				"description": "只展示指定阶段的任务，不传则展示所有阶段"
			},
			"status": {
				"type": "string",
				"description": "按状态过滤，多个状态用逗号分隔，可选值：pending（待处理）、running（进行中）、completed（已完成）、failed（失败）、skipped（跳过）"
			},
			"verbose": {
				"type": "boolean",
				"description": "仅在指定 task_id 时生效。为 true 时返回完整详情（含 detail 字段和关联任务文档内容）；默认 false 只返回基本信息"
			}
		}
	}`)
}

func (t *TaskListsTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		TaskID  uint   `json:"task_id"`
		Phase   int    `json:"phase"`
		Status  string `json:"status"`
		Verbose bool   `json:"verbose"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	wc := GetWorkContext(ctx)
	if wc == nil || wc.WorkspaceUUID == "" {
		return ErrorResult("工作区未设置，无法查询任务")
	}

	if params.TaskID > 0 {
		return t.executeDetailMode(ctx, wc, params.TaskID, params.Verbose)
	}

	allTasks, err := t.TaskService.ListWithSubTasks(wc.WorkspaceUUID)
	if err != nil {
		return ErrorResult("获取任务列表失败: " + err.Error())
	}

	// 过滤
	if params.Phase > 0 || params.Status != "" {
		var statusCodes []int
		if params.Status != "" {
			statusCodes = tasks.ParseStatusFilter(params.Status)
		}
		filtered := make([]model.Task, 0, len(allTasks))
		for _, tk := range allTasks {
			if params.Phase > 0 && tk.Phase != params.Phase {
				continue
			}
			if len(statusCodes) > 0 {
				match := false
				for _, sc := range statusCodes {
					if tk.Status == sc {
						match = true
						break
					}
				}
				if !match {
					continue
				}
			}
			filtered = append(filtered, tk)
		}
		allTasks = filtered
	}

	if len(allTasks) == 0 {
		return SuccessResult("暂无任务")
	}

	return SuccessResult(tasks.FormatTaskTree(allTasks))
}

// executeDetailMode 详情模式：输出任务详情 + 子任务 + 自动携带关联任务文档内容
func (t *TaskListsTool) executeDetailMode(ctx context.Context, wc *WorkContext, taskID uint, verbose bool) *Result {
	task, err := t.TaskService.GetByID(wc.WorkspaceUUID, taskID)
	if err != nil {
		return ErrorResult(fmt.Sprintf("任务 ID:%d 不存在", taskID))
	}

	subTasks, _ := t.TaskService.ListSubTasks(wc.WorkspaceUUID, taskID)

	var sb strings.Builder
	phaseLabel := ""
	if task.PhaseLabel != "" {
		phaseLabel = ": " + task.PhaseLabel
	}
	sb.WriteString(fmt.Sprintf("=== 阶段 #%d%s ===\n", task.Phase, phaseLabel))
	sb.WriteString(fmt.Sprintf("状态: %s", model.TaskStatusLabel(task.Status)))
	if task.TaskDoc != "" {
		sb.WriteString(fmt.Sprintf(" | 文档: %s", task.TaskDoc))
	}
	if task.DependsOn != "" {
		sb.WriteString(fmt.Sprintf(" | 依赖: #%s", task.DependsOn))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("标题: %s (ID:%d)\n", task.Title, task.ID))
	if task.Description != "" {
		sb.WriteString(fmt.Sprintf("描述: %s\n", task.Description))
	}
	if task.Detail != "" && verbose {
		sb.WriteString(fmt.Sprintf("详细说明: %s\n", task.Detail))
	}
	if task.Result != "" {
		sb.WriteString(fmt.Sprintf("结果: %s\n", task.Result))
	}

	if len(subTasks) > 0 {
		sb.WriteString("\n子任务:\n")
		for _, sub := range subTasks {
			subIcon := tasks.StatusIcon(sub.Status)
			subDocRef := ""
			if sub.TaskDoc != "" {
				subDocRef = fmt.Sprintf(" (文档:%s)", sub.TaskDoc)
			}
			sb.WriteString(fmt.Sprintf("  %s [%s] %s (ID:%d)%s\n", subIcon, model.TaskStatusLabel(sub.Status), sub.Title, sub.ID, subDocRef))
			if sub.Description != "" && sub.Status < 2 {
				sb.WriteString(fmt.Sprintf("    描述: %s\n", sub.Description))
			}
			if sub.Detail != "" && sub.Status < 2 && verbose {
				sb.WriteString(fmt.Sprintf("    详细说明: %s\n", sub.Detail))
			}
		}
	}

	if verbose && wc.WorkspaceDir != "" {
		docPaths := make(map[string]bool)
		if task.TaskDoc != "" {
			docPaths[task.TaskDoc] = true
		}
		for _, sub := range subTasks {
			if sub.TaskDoc != "" {
				docPaths[sub.TaskDoc] = true
			}
		}
		if len(docPaths) > 0 {
			sb.WriteString("\n--- 关联文档 ---\n")
			for docPath := range docPaths {
				fullPath := filepath.Join(wc.WorkspaceDir, docPath)
				data, err := os.ReadFile(fullPath)
				if err != nil {
					sb.WriteString(fmt.Sprintf("[%s] 读取失败: %s\n\n", docPath, err.Error()))
				} else {
					sb.WriteString(fmt.Sprintf("[%s]\n%s\n\n", docPath, string(data)))
				}
			}
		}
	}

	return SuccessResult(sb.String())
}

// ---- task_writers ----

// TaskWritersTool 创建/更新任务（自动推断操作类型）
type TaskWritersTool struct {
	TaskService TaskServicer
}

func (t *TaskWritersTool) Name() string { return "task_writers" }

func (t *TaskWritersTool) Description() string {
	return "创建或更新任务节点，顺序执行。自动推断操作：无 task_id 且无 parent_id 创建一级任务；无 task_id 但有 parent_id 创建子任务（继承父任务阶段）；有 task_id 则更新已有任务。同一批次可混合创建和更新。"
}

func (t *TaskWritersTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"tasks": {
				"type": "array",
				"description": "任务操作列表，顺序执行。每项根据 task_id/parent_id 自动判断操作类型",
				"items": {
					"type": "object",
					"properties": {
						"task_id": {"type": "integer", "description": "要更新的任务 ID（传入则为更新操作）"},
						"parent_id": {"type": "integer", "description": "父任务 ID（无 task_id 时传入则创建子任务，继承父任务阶段）"},
						"title": {"type": "string", "description": "任务标题（创建时必填，更新时可选）"},
						"description": {"type": "string", "description": "任务描述（可选）"},
						"detail": {"type": "string", "description": "任务详细说明（内联上下文，查询时自动携带，可选）"},
						"task_doc": {"type": "string", "description": "关联任务文档路径（如 tasks/backend-api.md，可选）"},
						"depends_on": {"type": "string", "description": "依赖的任务 ID（逗号分隔，同阶段内，可选）"},
						"phase": {"type": "integer", "description": "阶段编号（创建一级任务时使用，默认1）"},
						"phase_label": {"type": "string", "description": "阶段名称（可选）"},
						"priority": {"type": "integer", "description": "优先级：0=普通（默认）、1=高、2=紧急"},
						"assignee_id": {"type": "integer", "description": "分配的 Agent ID（可选）"},
						"required_skills": {"type": "string", "description": "所需技能关键词（逗号分隔，可选）"},
						"status": {"type": "integer", "description": "更新时的新状态：0=待处理，1=进行中，2=已完成，3=失败，4=跳过"},
						"result": {"type": "string", "description": "执行结果或失败原因（状态改为2或3时建议填写）"}
					}
				}
			}
		},
		"required": ["tasks"]
	}`)
}

type taskWriteItem struct {
	TaskID         uint    `json:"task_id"`
	ParentID       uint    `json:"parent_id"`
	Title          string  `json:"title"`
	Description    *string `json:"description"`
	Detail         *string `json:"detail"`
	TaskDoc        *string `json:"task_doc"`
	DependsOn      *string `json:"depends_on"`
	Phase          int     `json:"phase"`
	PhaseLabel     string  `json:"phase_label"`
	Priority       int     `json:"priority"`
	AssigneeID     uint    `json:"assignee_id"`
	RequiredSkills *string `json:"required_skills"`
	Status         *int    `json:"status"`
	Result         *string `json:"result"`
}

func (t *TaskWritersTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Tasks []taskWriteItem `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if len(params.Tasks) == 0 {
		return ErrorResult("tasks 不能为空")
	}

	wc := GetWorkContext(ctx)
	if wc == nil || wc.WorkspaceUUID == "" {
		return ErrorResult("工作区未设置")
	}

	results := make([]string, 0, len(params.Tasks))
	successCount := 0

	for i, item := range params.Tasks {
		var res string
		var isErr bool

		if item.TaskID > 0 {
			res, isErr = t.doUpdate(wc, item)
		} else if item.ParentID > 0 {
			res, isErr = t.doCreateSub(wc, item)
		} else {
			res, isErr = t.doCreate(wc, item, i)
		}

		if isErr {
			results = append(results, fmt.Sprintf("✗ %s", res))
		} else {
			results = append(results, fmt.Sprintf("✓ %s", res))
			successCount++
		}
	}

	n := len(params.Tasks)
	failCount := n - successCount
	if n == 1 {
		return &Result{Content: results[0][len("✓ "):], IsError: failCount > 0}
	}
	header := fmt.Sprintf("批量操作结果 (%d 项，%d 成功 / %d 失败)：", n, successCount, failCount)
	output := header + "\n" + strings.Join(results, "\n")
	return &Result{Content: output, IsError: failCount == n}
}

func (t *TaskWritersTool) doCreate(wc *WorkContext, item taskWriteItem, idx int) (string, bool) {
	if item.Title == "" {
		return fmt.Sprintf("第%d项 title 不能为空", idx+1), true
	}
	phase := item.Phase
	if phase <= 0 {
		phase = 1
	}
	desc := ""
	if item.Description != nil {
		desc = *item.Description
	}
	detail := ""
	if item.Detail != nil {
		detail = *item.Detail
	}
	taskDoc := ""
	if item.TaskDoc != nil {
		taskDoc = *item.TaskDoc
	}
	dependsOn := ""
	if item.DependsOn != nil {
		dependsOn = *item.DependsOn
	}
	reqSkills := ""
	if item.RequiredSkills != nil {
		reqSkills = *item.RequiredSkills
	}

	req := &model.TaskCreateReq{
		WorkspaceID:    wc.WorkspaceID,
		Title:          item.Title,
		Description:    desc,
		Detail:         detail,
		TaskDoc:        taskDoc,
		DependsOn:      dependsOn,
		Phase:          phase,
		PhaseLabel:     item.PhaseLabel,
		Priority:       item.Priority,
		AssigneeID:     item.AssigneeID,
		RequiredSkills: reqSkills,
	}
	task, err := t.TaskService.Create(wc.WorkspaceUUID, req)
	if err != nil {
		return fmt.Sprintf("创建任务失败: %s", err.Error()), true
	}
	return fmt.Sprintf("任务已创建: [ID:%d] %s（第%d阶段）", task.ID, task.Title, task.Phase), false
}

func (t *TaskWritersTool) doCreateSub(wc *WorkContext, item taskWriteItem) (string, bool) {
	if item.Title == "" {
		return "子任务 title 不能为空", true
	}
	desc := ""
	if item.Description != nil {
		desc = *item.Description
	}
	detail := ""
	if item.Detail != nil {
		detail = *item.Detail
	}
	taskDoc := ""
	if item.TaskDoc != nil {
		taskDoc = *item.TaskDoc
	}
	dependsOn := ""
	if item.DependsOn != nil {
		dependsOn = *item.DependsOn
	}
	reqSkills := ""
	if item.RequiredSkills != nil {
		reqSkills = *item.RequiredSkills
	}

	req := &model.TaskCreateReq{
		WorkspaceID:    wc.WorkspaceID,
		Title:          item.Title,
		Description:    desc,
		Detail:         detail,
		TaskDoc:        taskDoc,
		DependsOn:      dependsOn,
		Priority:       item.Priority,
		AssigneeID:     item.AssigneeID,
		RequiredSkills: reqSkills,
	}
	created, err := t.TaskService.Subdivide(wc.WorkspaceUUID, item.ParentID, []*model.TaskCreateReq{req})
	if err != nil {
		return fmt.Sprintf("创建子任务失败: %s", err.Error()), true
	}
	if len(created) > 0 {
		return fmt.Sprintf("子任务已创建: [ID:%d] %s（父任务 ID:%d）", created[0].ID, created[0].Title, item.ParentID), false
	}
	return "子任务创建异常", true
}

func (t *TaskWritersTool) doUpdate(wc *WorkContext, item taskWriteItem) (string, bool) {
	req := &model.TaskUpdateReq{
		Title:       item.Title,
		Description: item.Description,
		Detail:      item.Detail,
		TaskDoc:     item.TaskDoc,
		DependsOn:   item.DependsOn,
		Status:      item.Status,
		Result:      item.Result,
		PhaseLabel:  &item.PhaseLabel,
	}
	if item.Priority > 0 {
		req.Priority = &item.Priority
	}
	if item.AssigneeID > 0 {
		req.AssigneeID = &item.AssigneeID
	}
	if item.RequiredSkills != nil {
		req.RequiredSkills = item.RequiredSkills
	}
	// PhaseLabel 为空字符串时不更新
	if item.PhaseLabel == "" {
		req.PhaseLabel = nil
	}

	task, err := t.TaskService.Update(wc.WorkspaceUUID, item.TaskID, req)
	if err != nil {
		return fmt.Sprintf("更新任务 ID:%d 失败: %s", item.TaskID, err.Error()), true
	}
	return fmt.Sprintf("任务已更新: [ID:%d] %s → [%s]", task.ID, task.Title, model.TaskStatusLabel(task.Status)), false
}

// ---- task_deletes ----

// TaskDeletesTool 删除任务节点（支持批量，级联删除子任务）
type TaskDeletesTool struct {
	TaskService TaskServicer
}

func (t *TaskDeletesTool) Name() string { return "task_deletes" }

func (t *TaskDeletesTool) Description() string {
	return "删除指定任务节点（同时级联删除其所有子任务），顺序执行。不允许删除状态为「进行中」的任务。用于清理重复、无效或错误创建的任务。"
}

func (t *TaskDeletesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"task_ids": {
				"type": "array",
				"description": "要删除的任务 ID 列表，顺序执行",
				"items": {"type": "integer"}
			}
		},
		"required": ["task_ids"]
	}`)
}

func (t *TaskDeletesTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		TaskIDs []uint `json:"task_ids"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if len(params.TaskIDs) == 0 {
		return ErrorResult("task_ids 不能为空")
	}

	wc := GetWorkContext(ctx)
	if wc == nil || wc.WorkspaceUUID == "" {
		return ErrorResult("工作区未设置，无法删除任务")
	}

	results := make([]string, 0, len(params.TaskIDs))
	successCount := 0

	for _, id := range params.TaskIDs {
		title, subCount, err := t.TaskService.DeleteWithValidation(wc.WorkspaceUUID, id)
		if err != nil {
			results = append(results, fmt.Sprintf("✗ ID:%d — %s", id, err.Error()))
		} else {
			msg := fmt.Sprintf("任务已删除: [ID:%d] %s", id, title)
			if subCount > 0 {
				msg += fmt.Sprintf("（含 %d 个子任务）", subCount)
			}
			results = append(results, fmt.Sprintf("✓ %s", msg))
			successCount++
		}
	}

	n := len(params.TaskIDs)
	failCount := n - successCount
	if n == 1 {
		return &Result{Content: results[0][len("✓ "):], IsError: failCount > 0}
	}
	header := fmt.Sprintf("批量操作结果 (%d 项，%d 成功 / %d 失败)：", n, successCount, failCount)
	output := header + "\n" + strings.Join(results, "\n")
	return &Result{Content: output, IsError: failCount == n}
}
