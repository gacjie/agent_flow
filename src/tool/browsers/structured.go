package browsers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

func MarshalResult(v interface{}) (string, bool) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "序列化浏览器结果失败: " + err.Error(), true
	}
	return string(data), false
}

func Mark(sess *Session) Marker {
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	return Marker{Console: len(sess.ConsoleLogs), Net: len(sess.NetworkEvents)}
}

func DeltaFromSession(sess *Session, marker Marker) Delta {
	sess.Mu.Lock()
	logs := CloneLogs(sess.ConsoleLogs)
	events := CloneLogs(sess.NetworkEvents)
	doc := DocState{URL: strings.TrimSpace(sess.LastNavURL), Status: int(sess.LastNavStatus), MimeType: strings.TrimSpace(sess.LastNavMimeType)}
	sess.Mu.Unlock()
	if marker.Console < 0 || marker.Console > len(logs) {
		marker.Console = 0
	}
	if marker.Net < 0 || marker.Net > len(events) {
		marker.Net = 0
	}
	consoleErrors, consoleWarnings := SplitConsoleLogs(logs[marker.Console:])
	loadingFailures := LoadingFailures(events[marker.Net:])
	return Delta{
		Document:        doc,
		ConsoleErrors:   DedupeStrings(consoleErrors),
		ConsoleWarnings: DedupeStrings(consoleWarnings),
		LoadingFailures: DedupeStrings(loadingFailures),
		NetworkEvents:   DedupeStrings(events[marker.Net:]),
	}
}

func CollectStructuredState(sess *Session, p *Params, watch []string) (StructuredState, error) {
	var state StructuredState
	page := sess.Page.Timeout(15 * time.Second)
	plan := CapturePlan(p.OutputExpands)

	args, err := json.Marshal(map[string]interface{}{
		"watch_selectors":   watch,
		"snapshot_selector": p.SnapshotSelector,
		"structured_nodes":  plan.StructuredNodes,
		"structured_depth":  plan.StructuredDepth,
		"structured_refs":   plan.StructuredRefs,
	})
	if err != nil {
		return state, err
	}

	// 获取 title 和 url
	titleResult, err := page.Eval(`() => document.title`)
	if err != nil {
		return state, fmt.Errorf("获取 title 失败: %w", err)
	}
	state.Title = titleResult.Value.String()

	urlResult, err := page.Eval(`() => location.href`)
	if err != nil {
		return state, fmt.Errorf("获取 URL 失败: %w", err)
	}
	state.URL = urlResult.Value.String()

	// 执行 Inspector JS
	inspectExpr := fmt.Sprintf("() => (%s)(%s)", InspectorJS, string(args))
	payloadResult, err := page.Eval(inspectExpr)
	if err != nil {
		return state, fmt.Errorf("执行 Inspector 失败: %w", err)
	}

	var payload struct {
		Alerts             []string         `json:"alerts"`
		Validation         []string         `json:"validation"`
		KeyTexts           []string         `json:"key_texts"`
		Watched            []SelectorState  `json:"watched"`
		PageFrame          PageFrame        `json:"page_frame"`
		StructuredSnapshot string           `json:"structured_snapshot"`
		InteractionRefs    []InteractionRef `json:"interaction_refs"`
		ActiveElement      string           `json:"active_element"`
	}
	if err := json.Unmarshal([]byte(payloadResult.Value.JSON("", "")), &payload); err != nil {
		return state, fmt.Errorf("解析 Inspector 结果失败: %w", err)
	}

	// 获取页面源码（可选）
	if plan.IncludePageSource {
		srcResult, err := page.Eval(`() => document.documentElement ? document.documentElement.outerHTML : ""`)
		if err == nil {
			state.PageSource = srcResult.Value.String()
		}
	}

	state.Alerts = PickStrings(DedupeStrings(payload.Alerts), 6)
	state.Validation = PickStrings(DedupeStrings(payload.Validation), 6)
	state.KeyTexts = PickStrings(DedupeStrings(payload.KeyTexts), plan.KeyTexts)
	state.PageFrame = TrimPageFrame(payload.PageFrame, plan.PageForms, plan.PageFields)
	state.StructuredSnapshot = Truncate(payload.StructuredSnapshot, plan.StructuredChars)
	state.InteractionRefs = PickInteractionRefs(payload.InteractionRefs, plan.StructuredRefs)
	state.ActiveElement = strings.TrimSpace(payload.ActiveElement)
	state.Watched = PickStates(payload.Watched, plan.Watched)
	sess.Mu.Lock()
	sess.LastInteractionRefs = CloneInteractionRefs(state.InteractionRefs)
	sess.LastRefURL = state.URL
	sess.Mu.Unlock()
	return state, nil
}

func ResolveAction(a StructuredAction, before StructuredState) (StructuredAction, error) {
	resolved := a
	selector := strings.TrimSpace(a.Selector)
	if strings.HasPrefix(selector, "AUTO:") {
		return resolved, errors.New("当前浏览器工具不再支持 AUTO: 选择器，请先使用 navigate 获取 REF:rN 或直接传 CSS selector")
	}
	if strings.HasPrefix(selector, "REF:") {
		resolvedSelector, _, err := ResolveInteractionSelector(selector, before.InteractionRefs, before.URL)
		if err != nil {
			return resolved, err
		}
		resolved.Selector = resolvedSelector
	}
	return resolved, nil
}

func ExecuteAction(sess *Session, a StructuredAction) (interface{}, error) {
	page := sess.Page.Timeout(30 * time.Second)
	var evalResult interface{}
	switch a.Action {
	case "navigate":
		if strings.TrimSpace(a.URL) == "" {
			return nil, errors.New("navigate 缺少 url")
		}
		if err := page.Navigate(a.URL); err != nil {
			return nil, err
		}
		if err := page.WaitLoad(); err != nil {
			return nil, fmt.Errorf("等待页面加载失败: %w", err)
		}
	case "type":
		if strings.TrimSpace(a.Selector) == "" {
			return nil, errors.New("type 缺少 selector")
		}
		el, err := page.Element(a.Selector)
		if err != nil {
			return nil, fmt.Errorf("未找到元素 %s: %w", a.Selector, err)
		}
		if err := el.SelectAllText(); err != nil {
			return nil, err
		}
		if err := el.Input(a.Text); err != nil {
			return nil, err
		}
	case "click":
		if strings.TrimSpace(a.Selector) == "" {
			return nil, errors.New("click 缺少 selector")
		}
		el, err := page.Element(a.Selector)
		if err != nil {
			return nil, fmt.Errorf("未找到元素 %s: %w", a.Selector, err)
		}
		if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
			return nil, err
		}
		// click 可能触发导航，短暂等待页面加载完成
		_ = page.Timeout(3 * time.Second).WaitLoad()
	case "scroll":
		amount := a.Amount
		if amount == 0 {
			amount = 500
		}
		if strings.EqualFold(a.Direction, "up") {
			amount = -amount
		}
		if _, err := page.Eval(fmt.Sprintf("() => window.scrollBy(0, %d)", amount)); err != nil {
			return nil, err
		}
	case "evaluate":
		if strings.TrimSpace(a.Script) == "" {
			return nil, errors.New("evaluate 缺少 script")
		}
		result, err := page.Eval(fmt.Sprintf("() => { %s }", a.Script))
		if err != nil {
			return nil, err
		}
		evalResult = result.Value.Val()
	default:
		return nil, fmt.Errorf("不支持的 action: %s", a.Action)
	}
	if a.Wait > 0 {
		time.Sleep(time.Duration(a.Wait) * time.Second)
	}
	return evalResult, nil
}

func ShouldIncludeStructured(a StructuredAction, signals StepSignals, after StructuredState, d Delta, plan OutputPlan) bool {
	if plan.StructuredChars <= 0 || after.StructuredSnapshot == "" {
		return false
	}
	if plan.AlwaysIncludeStructured {
		return true
	}
	if a.Action == "navigate" || a.Action == "click" {
		return true
	}
	if signals.NavigationChanged {
		return true
	}
	if len(after.Validation) > 0 || len(d.ConsoleErrors) > 0 || len(d.LoadingFailures) > 0 {
		return true
	}
	return false
}

func ShouldIncludePageSource(plan OutputPlan, a StructuredAction, signals StepSignals, after StructuredState, d Delta, isLast bool) bool {
	if !plan.IncludePageSource || strings.TrimSpace(after.PageSource) == "" {
		return false
	}
	if isLast {
		return true
	}
	if a.Action == "navigate" {
		return true
	}
	if signals.NavigationChanged {
		return true
	}
	if len(after.Validation) > 0 || len(d.ConsoleErrors) > 0 || len(d.LoadingFailures) > 0 {
		return true
	}
	return false
}

func SummarizeStep(a StructuredAction, before, after StructuredState, d Delta, p *Params, evalResult interface{}, isLast bool, ms int64) StructuredStepResult {
	plan := SignalPlan(p.OutputExpands)
	signals := StepSignals{
		URL:               after.URL,
		Title:             after.Title,
		NavigationChanged: before.URL != "" && after.URL != "" && before.URL != after.URL,
		MainDocument:      d.Document,
		ActiveElement:     after.ActiveElement,
		EvalResult:        evalResult,
	}
	if plan.IncludePreviousContext || signals.NavigationChanged {
		signals.PreviousURL = before.URL
		signals.PreviousTitle = before.Title
	}
	if HasPageFrame(after.PageFrame) {
		frame := TrimPageFrame(after.PageFrame, plan.PageForms, plan.PageFields)
		signals.PageFrame = &frame
	}
	if ShouldIncludeStructured(a, signals, after, d, plan) {
		signals.StructuredSnapshot = Truncate(after.StructuredSnapshot, plan.StructuredChars)
		signals.InteractionRefs = PickInteractionRefs(after.InteractionRefs, plan.StructuredRefs)
	}
	if ShouldIncludePageSource(plan, a, signals, after, d, isLast) {
		signals.PageSource = after.PageSource
	}
	if len(after.Alerts) > 0 {
		signals.Alerts = PickStrings(after.Alerts, 3)
	}
	if len(after.Validation) > 0 {
		signals.Validation = PickStrings(after.Validation, 3)
	}
	if plan.KeyTexts > 0 && len(after.KeyTexts) > 0 {
		signals.KeyTexts = PickStrings(after.KeyTexts, plan.KeyTexts)
	}
	if plan.Watched > 0 && len(after.Watched) > 0 {
		signals.Watched = PickStates(after.Watched, plan.Watched)
	}
	if plan.ConsoleErrors > 0 && len(d.ConsoleErrors) > 0 {
		signals.ConsoleErrors = PickStrings(DedupeStrings(d.ConsoleErrors), plan.ConsoleErrors)
	}
	if plan.ConsoleWarnings > 0 && len(d.ConsoleWarnings) > 0 {
		signals.ConsoleWarnings = PickStrings(DedupeStrings(d.ConsoleWarnings), plan.ConsoleWarnings)
	}
	if plan.LoadingFailures > 0 && len(d.LoadingFailures) > 0 {
		signals.LoadingFailures = PickStrings(DedupeStrings(d.LoadingFailures), plan.LoadingFailures)
	}
	if plan.NetworkEvents > 0 && len(d.NetworkEvents) > 0 {
		signals.NetworkEvents = PickStrings(DedupeStrings(d.NetworkEvents), plan.NetworkEvents)
	}
	step := StructuredStepResult{Action: a.Action, Selector: a.Selector, ElapsedMS: ms, Signals: signals}
	parts := []string{"步骤: " + a.Action}
	if after.Title != "" {
		parts = append(parts, "标题: "+after.Title)
	}
	if after.URL != "" {
		parts = append(parts, "URL: "+after.URL)
	}
	if signals.NavigationChanged {
		parts = append(parts, "发生跳转")
	}
	if d.Document.Status > 0 {
		parts = append(parts, fmt.Sprintf("主文档状态: %d", d.Document.Status))
	}
	if len(signals.Validation) > 0 {
		parts = append(parts, fmt.Sprintf("表单校验: %d", len(signals.Validation)))
	}
	if len(signals.ConsoleErrors) > 0 {
		parts = append(parts, fmt.Sprintf("控制台错误: %d", len(signals.ConsoleErrors)))
	}
	if len(signals.LoadingFailures) > 0 {
		parts = append(parts, fmt.Sprintf("资源加载失败: %d", len(signals.LoadingFailures)))
	}
	if len(signals.InteractionRefs) > 0 {
		parts = append(parts, fmt.Sprintf("交互引用: %d", len(signals.InteractionRefs)))
	}
	if strings.TrimSpace(signals.PageSource) != "" {
		parts = append(parts, fmt.Sprintf("页面源码: %d chars", len(signals.PageSource)))
	}
	step.Summary = strings.Join(parts, " | ")
	return step
}

func ResolveFailureStep(index int, action StructuredAction, current StructuredState, doc DocState, err error) StructuredStepResult {
	msg := err.Error()
	return StructuredStepResult{
		Index:     index,
		Action:    action.Action,
		Selector:  action.Selector,
		OK:        false,
		ElapsedMS: 0,
		Summary:   fmt.Sprintf("第 %d 步失败：%s", index, msg),
		Error:     msg,
		Signals: StepSignals{
			URL:          current.URL,
			Title:        current.Title,
			MainDocument: doc,
		},
	}
}

func NormalizeWait(action string, wait int) int {
	if wait > 30 {
		return 30
	}
	if wait > 0 {
		return wait
	}
	switch action {
	case "navigate", "click", "type", "scroll":
		return 2
	default:
		return 0
	}
}

func BuildActions(p *Params) []StructuredAction {
	if len(p.Actions) > 0 {
		steps := make([]StructuredAction, 0, len(p.Actions))
		for _, item := range p.Actions {
			action := strings.ToLower(strings.TrimSpace(item.Action))
			wait := item.Wait
			if wait <= 0 {
				wait = p.Wait
			}
			steps = append(steps, StructuredAction{Action: action, URL: strings.TrimSpace(item.URL), Selector: strings.TrimSpace(item.Selector), Text: item.Text, Script: item.Script, Direction: strings.ToLower(strings.TrimSpace(item.Direction)), Amount: item.Amount, Wait: NormalizeWait(action, wait)})
		}
		return steps
	}
	action := strings.ToLower(strings.TrimSpace(p.Action))
	return []StructuredAction{{Action: action, URL: strings.TrimSpace(p.URL), Selector: strings.TrimSpace(p.Selector), Text: p.Text, Script: p.Script, Direction: strings.ToLower(strings.TrimSpace(p.Direction)), Amount: p.Amount, Wait: NormalizeWait(action, p.Wait)}}
}

func ClearSessionTelemetry(sess *Session) {
	sess.Mu.Lock()
	sess.ConsoleLogs = nil
	sess.NetworkEvents = nil
	sess.LastNavURL = ""
	sess.LastNavStatus = 0
	sess.LastNavMimeType = ""
	sess.LastInteractionRefs = nil
	sess.LastRefURL = ""
	sess.Mu.Unlock()
}

func CurrentDoc(sess *Session) DocState {
	if sess == nil {
		return DocState{}
	}
	sess.Mu.Lock()
	defer sess.Mu.Unlock()
	return DocState{URL: strings.TrimSpace(sess.LastNavURL), Status: int(sess.LastNavStatus), MimeType: strings.TrimSpace(sess.LastNavMimeType)}
}

func ExecuteStructured(sessionKey string, p *Params, workDir string) (string, bool) {
	started := time.Now()
	out := StructuredOutput{OK: true, StartedAt: started.Format(time.RFC3339), OutputExpands: PickStrings(p.OutputExpands, len(p.OutputExpands))}
	actions := BuildActions(p)
	if len(actions) == 0 || actions[0].Action == "" {
		out.OK = false
		out.FinishedAt = time.Now().Format(time.RFC3339)
		out.Summary = "未执行任何步骤"
		out.Error = "缺少 action 或 actions"
		return MarshalResult(out)
	}

	first := actions[0].Action
	var sess *Session
	var err error
	if first == "navigate" {
		sess, err = Mgr.GetOrCreate(sessionKey, p.Width, p.Height, workDir)
		if err != nil {
			out.OK = false
			out.FinishedAt = time.Now().Format(time.RFC3339)
			out.Summary = "浏览器启动失败"
			out.Error = err.Error()
			return MarshalResult(out)
		}
	} else {
		sess = Mgr.Get(sessionKey)
		if sess == nil {
			out.OK = false
			out.FinishedAt = time.Now().Format(time.RFC3339)
			out.Summary = "浏览器会话不存在，请先使用 navigate 打开页面"
			out.Error = out.Summary
			return MarshalResult(out)
		}
	}

	var current StructuredState
	if sess != nil {
		if state, stateErr := CollectStructuredState(sess, p, PickStrings(DedupeStrings(p.WatchSelectors), 12)); stateErr == nil {
			current = state
		}
	}
	if first == "navigate" {
		ClearSessionTelemetry(sess)
	}

	for i, action := range actions {
		stepStarted := time.Now()
		resolved, resolveErr := ResolveAction(action, current)
		if resolveErr != nil {
			step := ResolveFailureStep(i+1, action, current, CurrentDoc(sess), resolveErr)
			out.OK = false
			out.Steps = append(out.Steps, step)
			out.Summary = step.Summary
			out.Error = resolveErr.Error()
			out.FinishedAt = time.Now().Format(time.RFC3339)
			return MarshalResult(out)
		}
		marker := Mark(sess)
		evalResult, execErr := ExecuteAction(sess, resolved)
		after, stateErr := CollectStructuredState(sess, p, MergeWatchSelectors(p, resolved))
		if stateErr != nil {
			after = current
		}
		delta := DeltaFromSession(sess, marker)
		step := SummarizeStep(resolved, current, after, delta, p, evalResult, i == len(actions)-1 || execErr != nil || stateErr != nil, time.Since(stepStarted).Milliseconds())
		step.Index = i + 1
		if execErr == nil && stateErr != nil {
			execErr = stateErr
		}
		if execErr != nil {
			step.OK = false
			step.Error = execErr.Error()
			step.Summary = fmt.Sprintf("第 %d 步失败：%s", i+1, execErr.Error())
			out.OK = false
			out.Steps = append(out.Steps, step)
			out.Summary = step.Summary
			out.Error = execErr.Error()
			out.FinishedAt = time.Now().Format(time.RFC3339)
			return MarshalResult(out)
		}
		step.OK = true
		out.Steps = append(out.Steps, step)
		current = after
	}

	if len(out.Steps) == 0 {
		out.Summary = "未执行任何步骤"
	} else {
		out.Summary = out.Steps[len(out.Steps)-1].Summary
	}
	out.FinishedAt = time.Now().Format(time.RFC3339)
	return MarshalResult(out)
}
