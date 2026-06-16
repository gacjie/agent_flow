package tokenutil

import "agent_flow/src/provider"

// TokenSource 标识 token 数值来源
type TokenSource string

const (
	SourceAPI      TokenSource = "api"
	SourceEstimate TokenSource = "estimate"
)

// TokenInfo token 数值 + 来源
type TokenInfo struct {
	Tokens int         `json:"tokens"`
	Source TokenSource `json:"source"`
}

// Tracker 会话级 token 追踪器（非并发安全，单 goroutine 使用）
type Tracker struct {
	lastAPITokens    int
	lastMsgCount     int
	calibrationRatio float64
}

// NewTracker 创建追踪器
func NewTracker() *Tracker {
	return &Tracker{}
}

// RecordAPIUsage 记录 API 返回的 token 用量并更新校准系数
func (t *Tracker) RecordAPIUsage(totalTokens int, messages []provider.Message, toolDefs []provider.ToolDef) {
	t.lastAPITokens = totalTokens
	if t.calibrationRatio == 0 {
		localEst := EstimateContext(messages, toolDefs)
		if localEst > 0 {
			t.calibrationRatio = float64(totalTokens) / float64(localEst)
		}
	}
}

// RecordMessageCount 记录当前消息数（LLM 调用后调用）
func (t *Tracker) RecordMessageCount(count int) {
	t.lastMsgCount = count
}

// EstimateCurrent 估算当前上下文 token 数
// 优先使用 API 值 + 增量估算，无 API 值时使用本地估算（含校准）
func (t *Tracker) EstimateCurrent(messages []provider.Message, toolDefs []provider.ToolDef) TokenInfo {
	if t.lastAPITokens > 0 && t.lastMsgCount > 0 {
		estimated := t.lastAPITokens
		for i := t.lastMsgCount; i < len(messages); i++ {
			estimated += EstimateText(messages[i].Content)
			estimated += EstimateText(messages[i].ReasoningContent)
			for _, tc := range messages[i].ToolCalls {
				estimated += EstimateText(tc.Arguments) + len(tc.Name)
			}
		}
		return TokenInfo{Tokens: estimated, Source: SourceAPI}
	}
	estimated := EstimateContext(messages, toolDefs)
	if t.calibrationRatio > 0 {
		estimated = int(float64(estimated) * t.calibrationRatio)
	}
	return TokenInfo{Tokens: estimated, Source: SourceEstimate}
}

// Reset 重置追踪器（上下文整理后调用）
func (t *Tracker) Reset() {
	t.lastAPITokens = 0
	t.lastMsgCount = 0
	t.calibrationRatio = 0
}

// HasAPIData 是否已有 API 返回的 token 数据
func (t *Tracker) HasAPIData() bool {
	return t.lastAPITokens > 0
}

// CalibrationRatio 返回当前校准系数（0 表示未校准）
func (t *Tracker) CalibrationRatio() float64 {
	return t.calibrationRatio
}
