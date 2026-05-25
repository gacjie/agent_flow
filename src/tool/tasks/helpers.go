package tasks

import (
	"fmt"
	"strings"

	"agent_flow/src/model"
)

// StatusIcon 返回任务状态图标
func StatusIcon(status int) string {
	switch status {
	case 0:
		return "·"
	case 1:
		return "▶"
	case 2:
		return "✓"
	case 3:
		return "✗"
	case 4:
		return "-"
	default:
		return "?"
	}
}

// ParseStatusFilter 解析逗号分隔的状态过滤字符串
func ParseStatusFilter(statusStr string) []int {
	statusMap := map[string]int{
		"pending":   0,
		"running":   1,
		"completed": 2,
		"failed":    3,
		"skipped":   4,
	}
	var codes []int
	for _, s := range strings.Split(statusStr, ",") {
		s = strings.TrimSpace(strings.ToLower(s))
		if code, ok := statusMap[s]; ok {
			codes = append(codes, code)
		}
	}
	return codes
}

// GetSortedPhases 返回排序后的阶段编号列表
func GetSortedPhases(phaseGroups map[int][]model.Task) []int {
	phases := make([]int, 0, len(phaseGroups))
	for p := range phaseGroups {
		phases = append(phases, p)
	}
	for i := 0; i < len(phases); i++ {
		for j := i + 1; j < len(phases); j++ {
			if phases[i] > phases[j] {
				phases[i], phases[j] = phases[j], phases[i]
			}
		}
	}
	return phases
}

// FormatTaskTree 将任务列表格式化为树形结构字符串（支持任意深度）
func FormatTaskTree(allTasks []model.Task) string {
	phaseGroups := make(map[int][]model.Task)
	phaseLabels := make(map[int]string)
	for _, task := range allTasks {
		phaseGroups[task.Phase] = append(phaseGroups[task.Phase], task)
		if task.PhaseLabel != "" {
			phaseLabels[task.Phase] = task.PhaseLabel
		}
	}

	var sb strings.Builder
	phases := GetSortedPhases(phaseGroups)
	for _, phase := range phases {
		phaseTasks := phaseGroups[phase]
		label := phaseLabels[phase]
		if label != "" {
			sb.WriteString(fmt.Sprintf("## 第%d阶段：%s\n", phase, label))
		} else {
			sb.WriteString(fmt.Sprintf("## 第%d阶段\n", phase))
		}

		roots, childMap := model.BuildTaskTree(phaseTasks)
		for _, tk := range roots {
			renderTreeNode(&sb, tk, childMap, 0)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderTreeNode 递归渲染任务树节点
func renderTreeNode(sb *strings.Builder, tk model.Task, childMap map[uint][]model.Task, depth int) {
	indent := strings.Repeat("   ", depth)
	icon := StatusIcon(tk.Status)
	docRef := ""
	if tk.TaskDoc != "" {
		docRef = fmt.Sprintf(" [文档:%s]", tk.TaskDoc)
	}
	depRef := ""
	if tk.DependsOn != "" {
		depRef = fmt.Sprintf(" [依赖:#%s]", tk.DependsOn)
	}
	sb.WriteString(fmt.Sprintf("%s%s [%s] %s (ID:%d)%s%s\n", indent, icon, model.TaskStatusLabel(tk.Status), tk.Title, tk.ID, docRef, depRef))

	if tk.Description != "" && tk.Status < 2 {
		sb.WriteString(fmt.Sprintf("%s   描述: %s\n", indent, tk.Description))
	}
	if tk.Result != "" && tk.Status >= 2 {
		sb.WriteString(fmt.Sprintf("%s   结果: %s\n", indent, tk.Result))
	}

	if subs, ok := childMap[tk.ID]; ok {
		for _, sub := range subs {
			renderTreeNode(sb, sub, childMap, depth+1)
		}
	}
}
