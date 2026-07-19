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

// resolveContextPath 根据路径前缀将相对路径解析为绝对路径
// specs/、tasks/、uploads/ → WorkspaceDir
// skills/ → ContextRoot
// docs/ → ProjectPath
func resolveContextPath(ctx context.Context, path string) (string, error) {
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("不允许使用绝对路径: %s", path)
	}

	prefix := firstSegment(cleaned)
	switch prefix {
	case "specs", "tasks", "uploads":
		wc := GetWorkContext(ctx)
		if wc == nil || wc.WorkspaceDir == "" {
			return "", fmt.Errorf("工作区目录未设置")
		}
		fullPath := filepath.Join(wc.WorkspaceDir, cleaned)
		if !isSubPath(fullPath, wc.WorkspaceDir) {
			return "", fmt.Errorf("路径超出工作区范围: %s", path)
		}
		return fullPath, nil

	case "skills":
		contextRoot := config.Get().Agent.ContextRoot
		if contextRoot == "" {
			return "", fmt.Errorf("上下文根目录未配置")
		}
		fullPath := filepath.Join(contextRoot, cleaned)
		if !isSubPath(fullPath, contextRoot) {
			return "", fmt.Errorf("路径超出上下文目录范围: %s", path)
		}
		return fullPath, nil

	case "docs":
		wc := GetWorkContext(ctx)
		if wc == nil || wc.ProjectPath == "" {
			return "", fmt.Errorf("项目路径未设置，无法访问 docs/ 目录")
		}
		fullPath := filepath.Join(wc.ProjectPath, cleaned)
		docsDir := filepath.Join(wc.ProjectPath, "docs")
		if !isSubPath(fullPath, docsDir) {
			return "", fmt.Errorf("路径超出项目文档范围: %s", path)
		}
		return fullPath, nil

	default:
		return "", fmt.Errorf("不支持的路径前缀: %s，支持: specs/、tasks/、uploads/、skills/、docs/", prefix)
	}
}

// resolveListDir 根据 directory 解析列表目录的绝对路径
func resolveListDir(ctx context.Context, directory string) (string, error) {
	switch directory {
	case "skills":
		contextRoot := config.Get().Agent.ContextRoot
		if contextRoot == "" {
			return "", fmt.Errorf("上下文根目录未配置")
		}
		return filepath.Join(contextRoot, "skills"), nil

	case "specs", "tasks", "uploads":
		wc := GetWorkContext(ctx)
		if wc == nil || wc.WorkspaceDir == "" {
			return "", fmt.Errorf("工作区目录未设置")
		}
		return filepath.Join(wc.WorkspaceDir, directory), nil

	case "docs":
		wc := GetWorkContext(ctx)
		if wc == nil || wc.ProjectPath == "" {
			return "", fmt.Errorf("项目路径未设置，无法访问 docs/ 目录")
		}
		return filepath.Join(wc.ProjectPath, "docs"), nil

	default:
		return "", fmt.Errorf("不支持的 directory: %s，支持: skills、specs、tasks、uploads、docs", directory)
	}
}

// isReadOnlyPrefix 判断路径前缀是否为只读
func isReadOnlyPrefix(path string) bool {
	return firstSegment(filepath.Clean(path)) == "uploads"
}

// firstSegment 提取路径的第一段目录名
func firstSegment(cleaned string) string {
	cleaned = filepath.ToSlash(cleaned)
	if i := strings.IndexByte(cleaned, '/'); i >= 0 {
		return cleaned[:i]
	}
	return cleaned
}

func formatCtxSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
}

// ── ReadContextTool 读取上下文文档（支持批量）──────────────

type ReadContextTool struct{}

func (t *ReadContextTool) Name() string { return "read_contexts" }

func (t *ReadContextTool) Description() string {
	return "读取文档内容，并行读取。通过路径前缀自动路由：specs/、tasks/ → 工作区文档，uploads/ → 上传附件，skills/ → 上下文技能，docs/ → 项目文档。支持 offset/limit 按行读取。"
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
						"path": {"type": "string", "description": "文档路径，如 specs/requirement.md、skills/agent-creator.md、docs/api-guide.md、uploads/file.txt"},
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
		Files []struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if len(params.Files) == 0 {
		return ErrorResult("files 参数不能为空（至少提供一项）")
	}
	for i, f := range params.Files {
		if f.Path == "" {
			return ErrorResult(fmt.Sprintf("files[%d].path 不能为空", i))
		}
	}

	n := len(params.Files)
	results := make([]string, n)
	var wg sync.WaitGroup
	for i, f := range params.Files {
		wg.Add(1)
		go func(idx int, path string, offset, limit int) {
			defer wg.Done()
			fullPath, err := resolveContextPath(ctx, path)
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

// ── WriteContextTool 创建/编辑上下文文档（支持批量）───────

type WriteContextTool struct{}

func (t *WriteContextTool) Name() string { return "write_contexts" }

func (t *WriteContextTool) Description() string {
	return "创建、覆盖、追加或编辑文档，自动创建不存在的父目录。通过路径前缀自动路由：specs/、tasks/ → 工作区文档，skills/ → 上下文技能，docs/ → 项目文档。uploads/ 为只读，不可写入。\n\n写入模式：old_text 非空时搜索替换编辑（old_text 必须唯一匹配）；mode=\"append\" 时追加到文件末尾；否则覆盖整个文件。同一文件的多处编辑合并为原子读写。\n\n注意：单次调用不宜过多（写入建议单文件不超过 300 行，编辑建议不超过 10 处），超出请分多次调用。"
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
						"path": {"type": "string", "description": "文档路径，如 specs/requirement.md、skills/my-skill.md、docs/api-guide.md"},
						"content": {"type": "string", "description": "文件内容（写入/追加模式）或替换后的新文本（编辑模式）"},
						"old_text": {"type": "string", "description": "要替换的原始文本（提供此参数时为编辑模式，必须唯一匹配）"},
						"mode": {"type": "string", "enum": ["write", "append"], "description": "写入模式：write（默认，覆盖）/ append（追加到文件末尾）。old_text 非空时忽略此参数"}
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
		Mode    string `json:"mode"`
	}
	var params struct {
		Files []ctxFileItem `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if len(params.Files) == 0 {
		return ErrorResult("files 参数不能为空（至少提供一项）")
	}
	for i, f := range params.Files {
		if f.Path == "" {
			return ErrorResult(fmt.Sprintf("files[%d].path 不能为空", i))
		}
	}

	pathOrder := []string{}
	ctxFileOps := map[string][]writeOp{}
	for _, f := range params.Files {
		if isReadOnlyPrefix(f.Path) {
			return ErrorResult(fmt.Sprintf("uploads/ 为只读目录，不允许写入: %s", f.Path))
		}
		if _, exists := ctxFileOps[f.Path]; !exists {
			pathOrder = append(pathOrder, f.Path)
		}
		ctxFileOps[f.Path] = append(ctxFileOps[f.Path], writeOp{Content: f.Content, OldText: f.OldText, Mode: f.Mode})
	}

	n := len(pathOrder)
	results := make([]string, n)
	var wg sync.WaitGroup
	for i, path := range pathOrder {
		wg.Add(1)
		go func(idx int, filePath string, ops []writeOp) {
			defer wg.Done()
			res := processContextFileOps(ctx, filePath, ops)
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

func appendContextFile(fullPath, relPath, content string) *Result {
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ErrorResult("创建目录失败: " + err.Error())
	}
	existing, readErr := os.ReadFile(fullPath)
	if readErr == nil && len(existing) > 0 {
		content = string(existing) + "\n\n" + content
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return ErrorResult("写入文件失败: " + err.Error())
	}
	if readErr == nil {
		return SuccessResult(fmt.Sprintf("文档已追加: %s", relPath))
	}
	return SuccessResult(fmt.Sprintf("文档已创建: %s", relPath))
}

func processContextFileOps(ctx context.Context, path string, ops []writeOp) *Result {
	if path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := resolveContextPath(ctx, path)
	if err != nil {
		return ErrorResult(err.Error())
	}

	if len(ops) == 1 {
		op := ops[0]
		if op.OldText != "" {
			edits := []files.EditItem{{OldText: op.OldText, NewText: op.Content}}
			cnt, err := files.ApplyFileEdits(fullPath, edits)
			if err != nil {
				return ErrorResult(err.Error())
			}
			if cnt > 1 {
				return SuccessResult(fmt.Sprintf("文件已编辑（%d 处）: %s", cnt, path))
			}
			return SuccessResult("文件已编辑: " + path)
		}
		if op.Mode == "append" {
			return appendContextFile(fullPath, path, op.Content)
		}
		if err := files.WriteSingleFile(fullPath, op.Content); err != nil {
			return ErrorResult(err.Error())
		}
		return SuccessResult("文件已写入: " + path)
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
			if op.Mode == "append" {
				if content != "" {
					content = content + "\n\n" + op.Content
				} else {
					content = op.Content
				}
			} else {
				content = op.Content
			}
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
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ErrorResult("创建目录失败: " + err.Error())
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return ErrorResult("写入文件失败: " + err.Error())
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
	return "删除指定文档，并行删除。通过路径前缀自动路由：specs/、tasks/ → 工作区文档，skills/ → 上下文技能，docs/ → 项目文档。uploads/ 为只读，不可删除。仅允许删除文件，不允许删除目录。"
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
						"path": {"type": "string", "description": "文档路径，如 specs/old-spec.md、skills/old-skill.md、docs/old-doc.md"}
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
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if len(params.Files) == 0 {
		return ErrorResult("files 参数不能为空（至少提供一项）")
	}
	for i, f := range params.Files {
		if f.Path == "" {
			return ErrorResult(fmt.Sprintf("files[%d].path 不能为空", i))
		}
	}

	n := len(params.Files)
	results := make([]string, n)
	var wg sync.WaitGroup
	for i, f := range params.Files {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			if isReadOnlyPrefix(path) {
				results[idx] = fmt.Sprintf("✗ %s — uploads/ 为只读目录，不允许删除", path)
				return
			}
			fullPath, err := resolveContextPath(ctx, path)
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

// ── ContextListTool 列出文档 ───────────────────────

type ContextListTool struct{}

func (t *ContextListTool) Name() string { return "context_lists" }

func (t *ContextListTool) Description() string {
	return "列出上下文系统中的文档。支持的 directory：skills（上下文技能）、specs（工作区规格文档）、tasks（工作区任务文档）、uploads（上传附件）、docs（项目文档）。不指定时列出全部目录。"
}

func (t *ContextListTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"directory": {
				"type": "string",
				"enum": ["skills", "specs", "tasks", "uploads", "docs"],
				"description": "目录范围：skills（上下文技能）、specs（工作区规格文档）、tasks（工作区任务文档）、uploads（上传附件）、docs（项目文档）。不指定时列出全部目录"
			}
		}
	}`)
}

func (t *ContextListTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Directory string `json:"directory"`
	}
	if args != "" {
		json.Unmarshal([]byte(args), &params)
	}

	if params.Directory == "" {
		return t.listAll(ctx)
	}

	return t.listOne(ctx, params.Directory)
}

func (t *ContextListTool) listOne(ctx context.Context, directory string) *Result {
	dir, err := resolveListDir(ctx, directory)
	if err != nil {
		return ErrorResult(err.Error())
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return SuccessResult(fmt.Sprintf("%s/  （目录不存在）", directory))
		}
		return ErrorResult(fmt.Sprintf("读取目录失败: %s", readErr.Error()))
	}

	var items []string
	for _, e := range entries {
		if e.IsDir() {
			items = append(items, fmt.Sprintf("  %s/%s/", directory, e.Name()))
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, fmt.Sprintf("  %s/%s  (%s)", directory, e.Name(), formatCtxSize(info.Size())))
	}

	if len(items) == 0 {
		return SuccessResult(fmt.Sprintf("%s/  （空）", directory))
	}
	return SuccessResult(fmt.Sprintf("%s/ (%d 项):\n%s", directory, len(items), strings.Join(items, "\n")))
}

func (t *ContextListTool) listAll(ctx context.Context) *Result {
	dirs := []string{"skills", "specs", "tasks", "uploads", "docs"}
	var sections []string

	for _, d := range dirs {
		dir, err := resolveListDir(ctx, d)
		if err != nil {
			sections = append(sections, fmt.Sprintf("%s/  （不可用：%s）", d, err.Error()))
			continue
		}

		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				sections = append(sections, fmt.Sprintf("%s/  （目录不存在）", d))
			} else {
				sections = append(sections, fmt.Sprintf("%s/  （读取失败：%s）", d, readErr.Error()))
			}
			continue
		}

		var items []string
		for _, e := range entries {
			if e.IsDir() {
				items = append(items, fmt.Sprintf("  %s/%s/", d, e.Name()))
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			items = append(items, fmt.Sprintf("  %s/%s  (%s)", d, e.Name(), formatCtxSize(info.Size())))
		}

		if len(items) == 0 {
			sections = append(sections, fmt.Sprintf("%s/  （空）", d))
		} else {
			sections = append(sections, fmt.Sprintf("%s/ (%d 项):\n%s", d, len(items), strings.Join(items, "\n")))
		}
	}

	return SuccessResult(fmt.Sprintf("上下文目录总览 (%d 类):\n\n%s", len(dirs), strings.Join(sections, "\n\n")))
}

// ── SearchContextTool 搜索上下文内容 ─────────────────────

type SearchContextTool struct{}

func (t *SearchContextTool) Name() string { return "search_contexts" }

func (t *SearchContextTool) Description() string {
	return "在上下文系统中搜索文件内容，类似 grep，返回匹配行及行号。支持的 directory：skills（默认）、specs、tasks、docs。uploads/ 不支持搜索。"
}

func (t *SearchContextTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "搜索关键词（子串匹配）"},
			"directory": {"type": "string", "enum": ["skills", "specs", "tasks", "docs"], "description": "搜索范围：skills（默认）、specs、tasks、docs"},
			"file_pattern": {"type": "string", "description": "文件名 glob 过滤（如 *.md），可选"}
		},
		"required": ["pattern"]
	}`)
}

func (t *SearchContextTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Pattern     string `json:"pattern"`
		Directory   string `json:"directory"`
		FilePattern string `json:"file_pattern"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if params.Pattern == "" {
		return ErrorResult("搜索关键词不能为空")
	}

	if params.Directory == "" {
		params.Directory = "skills"
	}
	if params.Directory == "uploads" {
		return ErrorResult("uploads/ 不支持内容搜索")
	}

	searchDir, err := resolveListDir(ctx, params.Directory)
	if err != nil {
		return ErrorResult(err.Error())
	}

	var matches []string
	const maxMatches = 200

	_ = filepath.Walk(searchDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
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
			rel, _ := filepath.Rel(filepath.Dir(searchDir), p)
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

// ── AnalysisContextTool 分析文档结构（支持批量）─────

type AnalysisContextTool struct{}

func (t *AnalysisContextTool) Name() string { return "analysis_contexts" }

func (t *AnalysisContextTool) Description() string {
	return "分析文件结构，输出标题/代码块/链接等摘要信息（含行号），帮助理解文件内容而无需读取全文。通过路径前缀自动路由：specs/、tasks/ → 工作区文档，uploads/ → 上传附件，skills/ → 上下文技能，docs/ → 项目文档。各文件并行分析。"
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
						"path": {"type": "string", "description": "文档路径，如 specs/requirement.md、skills/agent-creator.md、docs/api-guide.md"}
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
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}
	if len(params.Files) == 0 {
		return ErrorResult("files 参数不能为空（至少提供一项）")
	}
	for i, f := range params.Files {
		if f.Path == "" {
			return ErrorResult(fmt.Sprintf("files[%d].path 不能为空", i))
		}
	}

	n := len(params.Files)
	results := make([]string, n)
	var wg sync.WaitGroup
	for i, f := range params.Files {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			fullPath, err := resolveContextPath(ctx, path)
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
