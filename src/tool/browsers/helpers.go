package browsers

import (
	"fmt"
	"sort"
	"strings"
)

func CloneInteractionRefs(src []InteractionRef) []InteractionRef {
	if len(src) == 0 {
		return nil
	}
	out := make([]InteractionRef, 0, len(src))
	for _, item := range src {
		ref := strings.TrimSpace(item.Ref)
		selector := strings.TrimSpace(item.Selector)
		if ref == "" || selector == "" {
			continue
		}
		out = append(out, InteractionRef{
			Ref:      ref,
			Selector: selector,
			Role:     strings.TrimSpace(item.Role),
			Name:     strings.TrimSpace(item.Name),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func FormatInteractionRefs(items []InteractionRef, limit int) []string {
	if len(items) == 0 {
		return nil
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		ref := strings.TrimSpace(item.Ref)
		selector := strings.TrimSpace(item.Selector)
		if ref == "" || selector == "" {
			continue
		}
		line := "REF:" + ref
		if role := strings.TrimSpace(item.Role); role != "" {
			line += " [" + role + "]"
		}
		if name := strings.TrimSpace(item.Name); name != "" {
			line += " " + name
		}
		line += " -> " + selector
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

func SnapshotRefContext(sess *Session) ([]InteractionRef, string) {
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	return CloneInteractionRefs(sess.LastInteractionRefs), strings.TrimSpace(sess.LastRefURL)
}

func ResolveInteractionSelector(selector string, refs []InteractionRef, pageURL string) (string, string, error) {
	raw := strings.TrimSpace(selector)
	if raw == "" {
		return "", "", fmt.Errorf("selector 不能为空")
	}
	if len(raw) < 4 || !strings.EqualFold(raw[:4], "REF:") {
		return raw, raw, nil
	}
	ref := strings.TrimSpace(raw[4:])
	if ref == "" {
		return "", raw, fmt.Errorf("引用选择器格式无效：%q，应为 REF:rN", raw)
	}
	token := "REF:" + ref
	if len(refs) == 0 {
		msg := "当前会话没有可用的 ref 映射，请先执行 navigate 获取包含 [ref=rN] 的结构化输出"
		if pageURL != "" {
			msg += fmt.Sprintf("（最近页面: %s）", pageURL)
		}
		return "", token, fmt.Errorf("%s", msg)
	}
	for _, item := range refs {
		if strings.EqualFold(strings.TrimSpace(item.Ref), ref) && strings.TrimSpace(item.Selector) != "" {
			resolved := strings.TrimSpace(item.Selector)
			return resolved, token + " -> " + resolved, nil
		}
	}
	msg := fmt.Sprintf("%s 在当前 ref 映射中不存在，可能页面已变化或 ref 已失效，请先重新执行 navigate 获取最新 ref", token)
	if pageURL != "" {
		msg += fmt.Sprintf("（最近页面: %s）", pageURL)
	}
	if available := FormatInteractionRefs(refs, 6); len(available) > 0 {
		msg += "。当前可用 refs: " + strings.Join(available, "; ")
	}
	return "", token, fmt.Errorf("%s", msg)
}

func FormatSection(sb *strings.Builder, title string, lines []string) {
	if len(lines) == 0 {
		return
	}
	if sb.Len() > 0 {
		sb.WriteString("\n")
	}
	sb.WriteString(title)
	sb.WriteString("\n")
	for _, line := range lines {
		sb.WriteString("- ")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
}

func NormalizeOutputExpand(v string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "default", "base":
		return "", nil
	case "console", "log", "logs", "console_logs":
		return "console", nil
	case "network", "net", "request", "requests", "network_events":
		return "network", nil
	case "page_source", "source", "html", "dom":
		return "page_source", nil
	default:
		return "", fmt.Errorf("不支持的扩展项 %q", strings.TrimSpace(v))
	}
}

func NormalizeOutputExpands(values []string) ([]string, error) {
	set := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, item := range values {
		normalized, err := NormalizeOutputExpand(item)
		if err != nil {
			return nil, err
		}
		if normalized == "" {
			continue
		}
		if _, ok := set[normalized]; ok {
			continue
		}
		set[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out, nil
}

func HasOutputExpand(values []string, target string) bool {
	for _, item := range values {
		if item == target {
			return true
		}
	}
	return false
}
func CapturePlan(expands []string) OutputPlan {
	plan := OutputPlan{KeyTexts: 12, Watched: 7, PageForms: 3, PageFields: 6, StructuredChars: 2800, StructuredRefs: 12, StructuredDepth: 6, StructuredNodes: 60}
	if HasOutputExpand(expands, "page_source") {
		plan.IncludePageSource = true
	}
	return plan
}

func SignalPlan(expands []string) OutputPlan {
	plan := OutputPlan{KeyTexts: 8, Watched: 5, PageForms: 2, PageFields: 4, StructuredChars: 1600, StructuredRefs: 10, ConsoleErrors: 2, LoadingFailures: 2, IncludePreviousContext: true}
	if HasOutputExpand(expands, "console") {
		plan.ConsoleErrors = 8
		plan.ConsoleWarnings = 8
	}
	if HasOutputExpand(expands, "network") {
		plan.LoadingFailures = 6
		plan.NetworkEvents = 8
	}
	if HasOutputExpand(expands, "page_source") {
		plan.IncludePageSource = true
	}
	return plan
}

func HasPageFrame(frame PageFrame) bool {
	return strings.TrimSpace(frame.MainHeading) != "" || len(frame.LayoutHints) > 0 || len(frame.Forms) > 0
}

func Truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "..."
}

func DedupeStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func PickStrings(items []string, n int) []string {
	if len(items) == 0 {
		return nil
	}
	if n > 0 && len(items) > n {
		items = items[:n]
	}
	out := make([]string, len(items))
	copy(out, items)
	return out
}

func PickStates(items []SelectorState, n int) []SelectorState {
	if len(items) == 0 {
		return nil
	}
	if n > 0 && len(items) > n {
		items = items[:n]
	}
	out := make([]SelectorState, len(items))
	copy(out, items)
	return out
}

func PickInteractionRefs(items []InteractionRef, n int) []InteractionRef {
	if len(items) == 0 {
		return nil
	}
	if n > 0 && len(items) > n {
		items = items[:n]
	}
	out := make([]InteractionRef, 0, len(items))
	for _, item := range items {
		out = append(out, InteractionRef{Ref: strings.TrimSpace(item.Ref), Selector: strings.TrimSpace(item.Selector), Role: strings.TrimSpace(item.Role), Name: strings.TrimSpace(item.Name)})
	}
	return out
}

func TrimPageFrame(frame PageFrame, formLimit int, fieldLimit int) PageFrame {
	trimmed := PageFrame{MainHeading: strings.TrimSpace(frame.MainHeading), LayoutHints: PickStrings(frame.LayoutHints, 4)}
	if len(frame.Forms) == 0 {
		return trimmed
	}
	forms := frame.Forms
	if formLimit > 0 && len(forms) > formLimit {
		forms = forms[:formLimit]
	}
	trimmed.Forms = make([]FormFrame, 0, len(forms))
	for _, form := range forms {
		item := FormFrame{Selector: strings.TrimSpace(form.Selector), Title: strings.TrimSpace(form.Title), Method: strings.TrimSpace(form.Method), Action: strings.TrimSpace(form.Action), FieldCount: form.FieldCount, SubmitTexts: PickStrings(form.SubmitTexts, 4)}
		fields := form.Fields
		if fieldLimit > 0 && len(fields) > fieldLimit {
			fields = fields[:fieldLimit]
		}
		if len(fields) > 0 {
			item.Fields = make([]FormField, 0, len(fields))
			for _, field := range fields {
				item.Fields = append(item.Fields, FormField{Selector: strings.TrimSpace(field.Selector), Label: strings.TrimSpace(field.Label), Name: strings.TrimSpace(field.Name), Type: strings.TrimSpace(field.Type), Placeholder: strings.TrimSpace(field.Placeholder), Required: field.Required, Filled: field.Filled})
			}
		}
		trimmed.Forms = append(trimmed.Forms, item)
	}
	return trimmed
}

func MergeWatchSelectors(p *Params, action StructuredAction) []string {
	set := map[string]struct{}{}
	for _, item := range p.WatchSelectors {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = struct{}{}
		}
	}
	if action.Selector != "" && !strings.HasPrefix(action.Selector, "REF:") && !strings.HasPrefix(action.Selector, "AUTO:") {
		set[action.Selector] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for item := range set {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func SplitConsoleLogs(items []string) ([]string, []string) {
	var errs []string
	var warns []string
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		switch {
		case strings.HasPrefix(lower, "[warning]"):
			warns = append(warns, trimmed)
		case strings.HasPrefix(lower, "[error]"), strings.HasPrefix(lower, "[exception]"):
			errs = append(errs, trimmed)
		}
	}
	return errs, warns
}

func LoadingFailures(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if strings.HasPrefix(strings.ToLower(trimmed), "[loading_failed]") {
			out = append(out, trimmed)
		}
	}
	return out
}

func ResolveDevice(device string, width, height int) (int, int, error) {
	device = strings.ToLower(strings.TrimSpace(device))
	if device == "" {
		return width, height, nil
	}
	preset, ok := DevicePresets[device]
	if !ok {
		names := make([]string, 0, len(DevicePresets))
		for k := range DevicePresets {
			names = append(names, k)
		}
		sort.Strings(names)
		return 0, 0, fmt.Errorf("未知设备预设 %q（支持: %s）", device, strings.Join(names, "/"))
	}
	return preset[0], preset[1], nil
}
