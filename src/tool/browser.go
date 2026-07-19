package tool

import (
	"context"
	"encoding/json"

	"agent_flow/src/tool/browsers"
)

type BrowserActionTool struct{}

func (t *BrowserActionTool) Name() string { return "browser_action" }

func (t *BrowserActionTool) Description() string {
	return `控制 headless 浏览器查看和操作网页。浏览器会话在多次调用间保持（空闲 5 分钟自动关闭）。
支持的 action：
- navigate: 打开 URL，返回结构化页面状态 JSON（默认含 page_frame / structured_snapshot / REF:rN / validation / key_texts）
- screenshot: 对当前页面截图，截图图片将以视觉内容返回供分析（同时保存到工作区 screenshots/ 目录，可在附件面板查看）
- click: 点击指定 CSS 选择器或 REF:rN 引用的元素
- type: 在指定输入框中输入文本，支持 CSS 选择器或 REF:rN 引用
- scroll: 滚动页面
- evaluate: 在页面上下文中执行 JavaScript 代码并返回结果（不支持 import/export 等 ES module 语法，请使用普通函数和 var/const/let）
- clear_cache: 清除浏览器 HTTP 缓存（保留 cookies/登录状态），下次 navigate 将获取最新资源
- resize: 调整浏览器视口大小，支持设备预设（mobile/tablet/desktop 等）或自定义 width/height，resize 后返回更新的页面状态
工具默认只输出对 AI 决策有用的必要事实，不输出整页源码，不替调用方判断"业务成功/失败"。
可通过 output_expands 扩展输出内容，支持一次传多个值，例如 console / network / page_source。
批量模式：使用 actions 数组可一次调用顺序执行多个交互操作（navigate/type/click/scroll/evaluate/resize），返回按步骤组织的结构化 JSON。`
}

func (t *BrowserActionTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["navigate", "screenshot", "click", "type", "scroll", "evaluate", "clear_cache", "resize"],
				"description": "操作类型（单步模式）"
			},
			"actions": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"action": {"type": "string", "enum": ["navigate", "type", "click", "scroll", "evaluate", "clear_cache", "resize"], "description": "操作类型"},
						"url": {"type": "string", "description": "页面地址（navigate 时必填）"},
						"selector": {"type": "string", "description": "CSS 选择器，或来自最近结构化输出的 REF:rN 引用"},
						"text": {"type": "string", "description": "输入文本（type 时必填）"},
						"script": {"type": "string", "description": "JavaScript 代码（evaluate 时必填）"},
						"direction": {"type": "string", "enum": ["up", "down"], "description": "滚动方向（默认 down）"},
						"amount": {"type": "integer", "description": "滚动像素（默认 500）"},
						"wait": {"type": "integer", "description": "本步操作后等待秒数（默认按 action 类型决定）"},
						"device": {"type": "string", "enum": ["mobile", "mobile_large", "tablet", "tablet_large", "desktop", "desktop_large"], "description": "设备预设（resize 时可选，优先于 width/height）"},
						"width": {"type": "integer", "description": "视口宽度（resize 时使用）"},
						"height": {"type": "integer", "description": "视口高度（resize 时使用）"}
					},
					"required": ["action"]
				},
				"description": "批量操作步骤数组，按顺序执行多个交互操作，返回按步骤组织的结构化 JSON。与 action 参数互斥，优先使用 actions"
			},
			"url": {"type": "string", "description": "页面地址（navigate 时必填）"},
			"selector": {"type": "string", "description": "CSS 选择器，或来自最近结构化输出的 REF:rN 引用"},
			"text": {"type": "string", "description": "输入文本（type 时必填）"},
			"script": {"type": "string", "description": "JavaScript 代码（evaluate 时必填）"},
			"direction": {"type": "string", "enum": ["up", "down"], "description": "滚动方向（默认 down）"},
			"amount": {"type": "integer", "description": "滚动像素（默认 500）"},
			"wait": {"type": "integer", "description": "操作后等待秒数（默认按 action 类型决定，最大 30）"},
			"width": {"type": "integer", "description": "视口宽度（默认 1280，navigate 创建会话时或 resize 时生效）"},
			"height": {"type": "integer", "description": "视口高度（默认 720，navigate 创建会话时或 resize 时生效）"},
			"device": {"type": "string", "enum": ["mobile", "mobile_large", "tablet", "tablet_large", "desktop", "desktop_large"], "description": "设备预设（resize 时可选，优先于 width/height。mobile=375x667, mobile_large=428x926, tablet=768x1024, tablet_large=1024x1366, desktop=1280x720, desktop_large=1920x1080）"},
			"output_expands": {"type": "array", "items": {"type": "string", "enum": ["console", "network", "page_source"]}, "description": "扩展输出字段列表，可同时传多个值；例如 console / network / page_source"},
			"snapshot_selector": {"type": "string", "description": "结构化快照优先截取的根节点 CSS 选择器"},
			"watch_selectors": {"type": "array", "items": {"type": "string"}, "description": "需要额外观测状态的 CSS 选择器列表"}
		}
	}`)
}

func (t *BrowserActionTool) Execute(ctx context.Context, args string) *Result {
	var p browsers.Params
	if err := json.Unmarshal([]byte(args), &p); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	if p.Wait > 30 {
		p.Wait = 30
	}
	if p.Width <= 0 {
		p.Width = 1280
	}
	if p.Height <= 0 {
		p.Height = 720
	}
	var err error
	p.OutputExpands, err = browsers.NormalizeOutputExpands(p.OutputExpands)
	if err != nil {
		return ErrorResult("output_expands 参数无效: " + err.Error())
	}

	sessionKey := "_default"
	wc := GetWorkContext(ctx)
	if wc != nil && wc.WorkspaceUUID != "" {
		sessionKey = wc.WorkspaceUUID
	}

	workspaceDir := ""
	if wc != nil {
		workspaceDir = wc.WorkspaceDir
	}

	if len(p.Actions) > 0 {
		content, isErr := browsers.ExecuteStructured(sessionKey, &p, workspaceDir)
		return &Result{Content: content, IsError: isErr}
	}
	if p.Action == "" {
		return ErrorResult("需要 action 或 actions 参数")
	}

	switch p.Action {
	case "navigate", "click", "type", "scroll", "evaluate", "clear_cache", "resize":
		content, isErr := browsers.ExecuteStructured(sessionKey, &p, workspaceDir)
		return &Result{Content: content, IsError: isErr}
	case "screenshot":
		sr := browsers.DoScreenshot(sessionKey, workspaceDir)
		result := &Result{Content: sr.Message, IsError: sr.IsError}
		if !sr.IsError && sr.Data != "" {
			result.Images = []ImageData{{
				Path:      sr.FilePath,
				MediaType: "image/png",
				Data:      sr.Data,
			}}
		}
		return result
	default:
		return ErrorResult("未知 action: " + p.Action + "（支持: navigate/screenshot/click/type/scroll/evaluate/clear_cache/resize）")
	}
}
