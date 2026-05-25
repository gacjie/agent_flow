package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"agent_flow/src/config"
	"agent_flow/src/tool/files"
)

// ── 公共辅助 ─────────────────────────────────────────────

// contextPath 将相对路径解析为 context 目录下的绝对路径并校验安全性
func contextPath(path string) (string, error) {
	contextRoot := config.Get().Agent.ContextRoot
	if contextRoot == "" {
		return "", fmt.Errorf("上下文根目录未配置")
	}

	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("不允许使用绝对路径: %s", path)
	}
	fullPath := filepath.Join(contextRoot, cleaned)

	if !isSubPath(fullPath, contextRoot) {
		return "", fmt.Errorf("路径超出上下文目录范围: %s", path)
	}

	agentsDir := filepath.Join(contextRoot, "agents")
	if isSubPath(fullPath, agentsDir) {
		return "", fmt.Errorf("禁止通过此工具访问 agents/ 目录，智能体配置由专用工具管理")
	}

	return fullPath, nil
}

// ── ReadContextTool 读取上下文文档（支持批量）──────────────

type ReadContextTool struct{}

func (t *ReadContextTool) Name() string { return "read_contexts" }

func (t *ReadContextTool) Description() string {
	return "读取上下文目录（context/）中的文档内容，并行读取。路径相对于上下文目录，如 skills/agent-creator.md。支持 offset/limit 按行读取。禁止访问 agents/ 目录。"
}

func (t *ReadContextTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要读取的文档列表，并行读取",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文档路径（相对于 context 目录），如 skills/agent-creator.md"},
						"offset": {"type": "integer", "description": "起始行号（从1开始，可选）"},
						"limit": {"type": "integer", "description": "读取行数（可选）"}
					},
					"required": ["path"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *ReadContextTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
		Files  []struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	if len(params.Files) > 0 {
		n := len(params.Files)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, f := range params.Files {
			wg.Add(1)
			go func(idx int, path string, offset, limit int) {
				defer wg.Done()
				fullPath, err := contextPath(path)
				if err != nil {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s（错误）===\n%s", idx+1, n, path, err.Error())
					return
				}
				content, err := files.ReadSingleFile(fullPath, offset, limit)
				if err != nil {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s（错误）===\n%s", idx+1, n, path, err.Error())
				} else {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s ===\n%s", idx+1, n, path, content)
				}
			}(i, f.Path, f.Offset, f.Limit)
		}
		wg.Wait()
		return SuccessResult(strings.Join(results, "\n\n"))
	}

	if params.Path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := contextPath(params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	content, err := files.ReadSingleFile(fullPath, params.Offset, params.Limit)
	if err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult(content)
}

// ── WriteContextTool 创建/编辑上下文文档（支持批量）───────

type WriteContextTool struct{}

func (t *WriteContextTool) Name() string { return "write_contexts" }

func (t *WriteContextTool) Description() string {
	return "在上下文目录（context/）创建、覆盖或编辑文档，自动创建不存在的父目录。当 old_text 为空时创建或覆盖整个文件；当 old_text 非空时搜索替换编辑（old_text 必须在文件中唯一匹配）。同一文件的多处编辑合并为原子读写。禁止操作 agents/ 目录。\n\n注意：单次调用不宜过多（写入建议单文件不超过 300 行，编辑建议不超过 10 处），超出请分多次调用。"
}

func (t *WriteContextTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "文件操作列表。不同文件并行处理；同一文件的多处编辑按顺序原子执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文档路径（相对于 context 目录），如 skills/my-skill.md"},
						"content": {"type": "string", "description": "文件内容（写入模式）或替换后的新文本（编辑模式）"},
						"old_text": {"type": "string", "description": "要替换的原始文本（提供此参数时为编辑模式，必须唯一匹配）"}
					},
					"required": ["path", "content"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *WriteContextTool) Execute(ctx context.Context, args string) *Result {
	type ctxFileItem struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		OldText string `json:"old_text"`
	}
	var params struct {
		Path    string        `json:"path"`
		Content string        `json:"content"`
		OldText string        `json:"old_text"`
		Files   []ctxFileItem `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	if len(params.Files) > 0 {
		pathOrder := []string{}
		ctxFileOps := map[string][]writeOp{}
		for _, f := range params.Files {
			if _, exists := ctxFileOps[f.Path]; !exists {
				pathOrder = append(pathOrder, f.Path)
			}
			ctxFileOps[f.Path] = append(ctxFileOps[f.Path], writeOp{f.Content, f.OldText})
		}

		n := len(pathOrder)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, path := range pathOrder {
			wg.Add(1)
			go func(idx int, filePath string, ops []writeOp) {
				defer wg.Done()
				res := processContextFileOps(filePath, ops)
				if res.IsError {
					results[idx] = fmt.Sprintf("✗ %s — %s", filePath, res.Content)
				} else {
					results[idx] = fmt.Sprintf("✓ %s — %s", filePath, res.Content)
				}
			}(i, path, ctxFileOps[path])
		}
		wg.Wait()

		successCount := 0
		for _, r := range results {
			if strings.HasPrefix(r, "✓") {
				successCount++
			}
		}
		failCount := n - successCount
		header := fmt.Sprintf("批量操作结果 (%d 个文件，%d 成功 / %d 失败)：", n, successCount, failCount)
		output := header + "\n" + strings.Join(results, "\n")
		if failCount == n {
			return ErrorResult(output)
		}
		return SuccessResult(output)
	}

	if params.Path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := contextPath(params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	if params.OldText != "" {
		edits := []files.EditItem{{OldText: params.OldText, NewText: params.Content}}
		cnt, err := files.ApplyFileEdits(fullPath, edits)
		if err != nil {
			return ErrorResult(err.Error())
		}
		if cnt > 1 {
			return SuccessResult(fmt.Sprintf("文件已编辑（%d 处）: %s", cnt, params.Path))
		}
		return SuccessResult("文件已编辑: " + params.Path)
	}
	if err := files.WriteSingleFile(fullPath, params.Content); err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult("文件已写入: " + params.Path)
}

func processContextFileOps(path string, ops []writeOp) *Result {
	if path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := contextPath(path)
	if err != nil {
		return ErrorResult(err.Error())
	}

	if len(ops) == 1 {
		if ops[0].OldText == "" {
			if err := files.WriteSingleFile(fullPath, ops[0].Content); err != nil {
				return ErrorResult(err.Error())
			}
			return SuccessResult("文件已写入: " + path)
		}
		edits := []files.EditItem{{OldText: ops[0].OldText, NewText: ops[0].Content}}
		cnt, err := files.ApplyFileEdits(fullPath, edits)
		if err != nil {
			return ErrorResult(err.Error())
		}
		if cnt > 1 {
			return SuccessResult(fmt.Sprintf("文件已编辑（%d 处）: %s", cnt, path))
		}
		return SuccessResult("文件已编辑: " + path)
	}

	allEdits := true
	for _, op := range ops {
		if op.OldText == "" {
			allEdits = false
			break
		}
	}
	if allEdits {
		edits := make([]files.EditItem, len(ops))
		for i, op := range ops {
			edits[i] = files.EditItem{OldText: op.OldText, NewText: op.Content}
		}
		cnt, err := files.ApplyFileEdits(fullPath, edits)
		if err != nil {
			return ErrorResult(err.Error())
		}
		return SuccessResult(fmt.Sprintf("文件已编辑（%d 处）: %s", cnt, path))
	}

	// 混合操作
	content := ""
	if data, readErr := os.ReadFile(fullPath); readErr == nil {
		content = string(data)
	}
	editCount := 0
	for i, op := range ops {
		if op.OldText == "" {
			content = op.Content
		} else {
			count := strings.Count(content, op.OldText)
			if count == 0 {
				return ErrorResult(fmt.Sprintf("第%d处编辑失败：未找到匹配的文本", i+1))
			}
			if count > 1 {
				return ErrorResult(fmt.Sprintf("第%d处编辑失败：找到 %d 处匹配，old_text 必须唯一匹配", i+1, count))
			}
			content = strings.Replace(content, op.OldText, op.Content, 1)
			editCount++
		}
	}
	if err := files.WriteSingleFile(fullPath, content); err != nil {
		return ErrorResult(err.Error())
	}
	if editCount > 0 {
		return SuccessResult(fmt.Sprintf("文件已更新（%d 处编辑）: %s", editCount, path))
	}
	return SuccessResult("文件已写入: " + path)
}

// ── DeleteContextTool 删除上下文文档（支持批量）──────────

type DeleteContextTool struct{}

func (t *DeleteContextTool) Name() string { return "delete_contexts" }

func (t *DeleteContextTool) Description() string {
	return "删除上下文目录（context/）中指定的文档，并行删除。仅允许删除文件，不允许删除目录。禁止操作 agents/ 目录。"
}

func (t *DeleteContextTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要删除的文档路径列表，并行删除",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文档路径（相对于 context 目录），如 skills/old-skill.md"}
					},
					"required": ["path"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *DeleteContextTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Path  string `json:"path"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	if len(params.Files) > 0 {
		n := len(params.Files)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, f := range params.Files {
			wg.Add(1)
			go func(idx int, path string) {
				defer wg.Done()
				fullPath, err := contextPath(path)
				if err != nil {
					results[idx] = fmt.Sprintf("✗ %s — %s", path, err.Error())
					return
				}
				info, sErr := os.Stat(fullPath)
				if sErr != nil {
					results[idx] = fmt.Sprintf("✗ %s — 文件不存在: %s", path, sErr.Error())
					return
				}
				if info.IsDir() {
					results[idx] = fmt.Sprintf("✗ %s — 不允许删除目录", path)
					return
				}
				if rErr := os.Remove(fullPath); rErr != nil {
					results[idx] = fmt.Sprintf("✗ %s — 删除失败: %s", path, rErr.Error())
				} else {
					results[idx] = fmt.Sprintf("✓ %s — 已删除", path)
				}
			}(i, f.Path)
		}
		wg.Wait()

		successCount := 0
		for _, r := range results {
			if strings.HasPrefix(r, "✓") {
				successCount++
			}
		}
		failCount := n - successCount
		header := fmt.Sprintf("批量删除结果 (%d 项，%d 成功 / %d 失败)：", n, successCount, failCount)
		output := header + "\n" + strings.Join(results, "\n")
		if failCount == n {
			return ErrorResult(output)
		}
		return SuccessResult(output)
	}

	if params.Path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := contextPath(params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return ErrorResult("文件不存在: " + err.Error())
	}
	if info.IsDir() {
		return ErrorResult("不允许删除目录: " + params.Path)
	}
	if err := os.Remove(fullPath); err != nil {
		return ErrorResult("删除失败: " + err.Error())
	}
	return SuccessResult("已删除: " + params.Path)
}

// ── ContextListTool 列出上下文文档 ───────────────────────

type ContextListTool struct{}

func (t *ContextListTool) Name() string { return "context_lists" }

func (t *ContextListTool) Description() string {
	return "列出上下文目录（context/）中的文档。默认列出 skills/ 目录下全部文件，可通过 scope 参数指定其他子目录。排除 agents/ 目录（智能体配置由专用工具管理）。"
}

func (t *ContextListTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"scope": {
				"type": "string",
				"description": "可选，指定只列出某个子目录（如 skills），不传则默认列出 skills/ 目录"
			}
		}
	}`)
}

func (t *ContextListTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Scope string `json:"scope"`
	}
	if args != "" {
		json.Unmarshal([]byte(args), &params)
	}

	contextRoot := config.Get().Agent.ContextRoot
	if contextRoot == "" {
		return ErrorResult("上下文根目录未配置")
	}

	if params.Scope == "" {
		params.Scope = "skills"
	}
	if params.Scope == "agents" {
		return ErrorResult("禁止通过此工具访问 agents/ 目录，智能体配置由专用工具管理")
	}

	return listContextScope(contextRoot, params.Scope)
}

func listContextScope(contextRoot, scope string) *Result {
	dir := filepath.Join(contextRoot, scope)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return SuccessResult(fmt.Sprintf("%s/  （目录不存在）", scope))
		}
		return ErrorResult(fmt.Sprintf("读取目录失败: %s", err.Error()))
	}

	var items []string
	for _, e := range entries {
		if e.IsDir() {
			items = append(items, fmt.Sprintf("  %s/%s/", scope, e.Name()))
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, fmt.Sprintf("  %s/%s  (%s)", scope, e.Name(), formatCtxSize(info.Size())))
	}

	if len(items) == 0 {
		return SuccessResult(fmt.Sprintf("%s/  （空）", scope))
	}
	return SuccessResult(fmt.Sprintf("%s/ (%d 项):\n%s", scope, len(items), strings.Join(items, "\n")))
}

func formatCtxSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
}

// ── SearchContextTool 搜索上下文内容 ─────────────────────

type SearchContextTool struct{}

func (t *SearchContextTool) Name() string { return "search_contexts" }

func (t *SearchContextTool) Description() string {
	return "在上下文目录（context/）中搜索文件内容，类似 grep，返回匹配行及行号。排除 agents/ 目录。"
}

func (t *SearchContextTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "搜索关键词（子串匹配）"},
			"scope": {"type": "string", "description": "可选，限定搜索的子目录（如 skills），不传则搜索整个 context/ 目录（排除 agents/）"},
			"file_pattern": {"type": "string", "description": "文件名 glob 过滤（如 *.md），可选"}
		},
		"required": ["pattern"]
	}`)
}

func (t *SearchContextTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Pattern     string `json:"pattern"`
		Scope       string `json:"scope"`
		FilePattern string `json:"file_pattern"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if params.Pattern == "" {
		return ErrorResult("搜索关键词不能为空")
	}
	if params.Scope == "agents" {
		return ErrorResult("禁止通过此工具搜索 agents/ 目录")
	}

	contextRoot := config.Get().Agent.ContextRoot
	if contextRoot == "" {
		return ErrorResult("上下文根目录未配置")
	}

	searchDir := contextRoot
	if params.Scope != "" {
		searchDir = filepath.Join(contextRoot, params.Scope)
	}
	agentsDir := filepath.Join(contextRoot, "agents")

	var matches []string
	const maxMatches = 200

	_ = filepath.Walk(searchDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				absP, _ := filepath.Abs(p)
				absAgents, _ := filepath.Abs(agentsDir)
				if absP == absAgents {
					return filepath.SkipDir
				}
				if strings.HasPrefix(info.Name(), ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") || info.Size() > 1024*1024 {
			return nil
		}
		if params.FilePattern != "" {
			matched, _ := filepath.Match(params.FilePattern, info.Name())
			if !matched {
				return nil
			}
		}
		fileMatches := files.SearchInFile(p, params.Pattern)
		if len(fileMatches) > 0 {
			rel, _ := filepath.Rel(contextRoot, p)
			for _, m := range fileMatches {
				matches = append(matches, fmt.Sprintf("%s:%s", rel, m))
				if len(matches) >= maxMatches {
					return fmt.Errorf("max_reached")
				}
			}
		}
		return nil
	})

	if len(matches) == 0 {
		return SuccessResult("未找到匹配内容")
	}
	output := strings.Join(matches, "\n")
	if len(matches) >= maxMatches {
		output += fmt.Sprintf("\n... (结果已截断，最多显示 %d 条)", maxMatches)
	}
	return SuccessResult(output)
}

// ── AnalysisContextTool 分析上下文文档结构（支持批量）─────

type AnalysisContextTool struct{}

func (t *AnalysisContextTool) Name() string { return "analysis_contexts" }

func (t *AnalysisContextTool) Description() string {
	return "分析上下文目录（context/）中的文件结构，输出标题/代码块/链接等摘要信息（含行号），帮助理解文件内容而无需读取全文。路径相对于上下文目录，如 skills/agent-creator.md。禁止访问 agents/ 目录。各文件并行分析。"
}

func (t *AnalysisContextTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要分析的文档列表，并行执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文档路径（相对于 context 目录），如 skills/agent-creator.md"}
					},
					"required": ["path"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *AnalysisContextTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Path  string `json:"path"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	if len(params.Files) > 0 {
		n := len(params.Files)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, f := range params.Files {
			wg.Add(1)
			go func(idx int, path string) {
				defer wg.Done()
				fullPath, err := contextPath(path)
				if err != nil {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s（错误）===\n%s", idx+1, n, path, err.Error())
					return
				}
				content, err := files.AnalyzeSingleFile(fullPath)
				if err != nil {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s（错误）===\n%s", idx+1, n, path, err.Error())
				} else {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s ===\n%s", idx+1, n, path, content)
				}
			}(i, f.Path)
		}
		wg.Wait()
		return SuccessResult(strings.Join(results, "\n\n"))
	}

	if params.Path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := contextPath(params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	content, err := files.AnalyzeSingleFile(fullPath)
	if err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult(content)
}
