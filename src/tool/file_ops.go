package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"agent_flow/src/tool/files"
)

// ── 公共辅助 ─────────────────────────────────────────────

// safePath 将相对路径解析为绝对路径并校验安全性
func safePath(ctx context.Context, path string) (string, error) {
	wc := GetWorkContext(ctx)
	if wc == nil {
		return "", fmt.Errorf("未设置工作目录上下文")
	}

	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = filepath.Clean(path)
	} else {
		fullPath = filepath.Clean(filepath.Join(wc.WorkDir, path))
	}

	if isSubPath(fullPath, wc.WorkDir) {
		return fullPath, nil
	}
	for _, dir := range wc.AllowedDirs {
		if isSubPath(fullPath, dir) {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("路径超出允许范围: %s", path)
}

// isSubPath 检查 path 是否为 base 的子路径
func isSubPath(path, base string) bool {
	absPath, _ := filepath.Abs(path)
	absBase, _ := filepath.Abs(base)
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

// writeOp 写入工具的单项操作
type writeOp struct {
	Content string
	OldText string
}

// fileOp 文件操作项（context_ops 等共用）
type fileOp = writeOp

// editItem 编辑操作项（context_ops 等共用）
type editItem = files.EditItem

// ── ReadFileTool 读取文件内容（支持批量）─────────────────

type ReadFileTool struct{}

func (t *ReadFileTool) Name() string { return "read_files" }

func (t *ReadFileTool) Description() string {
	return "读取项目目录下文件的内容，各文件并行读取，结果以分隔块格式返回。支持 offset/limit 按行读取。注意：此工具操作项目代码目录，无法读取工作区文档（工作区文档由系统自动加载到上下文）。"
}

func (t *ReadFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要读取的文件列表，并行读取",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文件路径（相对于项目目录）"},
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

func (t *ReadFileTool) Execute(ctx context.Context, args string) *Result {
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
				fullPath, err := safePath(ctx, path)
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
	fullPath, err := safePath(ctx, params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	content, err := files.ReadSingleFile(fullPath, params.Offset, params.Limit)
	if err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult(content)
}

// ── WriteFileTool 文件写入与编辑（支持批量）──────────────
type WriteFileTool struct{}

func (t *WriteFileTool) Name() string { return "write_files" }

func (t *WriteFileTool) Description() string {
	return "创建、覆盖或编辑项目目录下的文件，自动创建不存在的父目录。当 old_text 为空时创建或覆盖整个文件；当 old_text 非空时搜索替换编辑（old_text 必须在文件中唯一匹配）。同一文件的多处编辑合并为原子读写（按数组顺序依次应用），不同文件并行处理。此工具操作项目代码目录，写工作文档请使用 write_work_docs。\n\n注意：单次调用不宜过多（写入建议单文件不超过 300 行，编辑建议不超过 10 处），超出请分多次调用。"
}

func (t *WriteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "文件操作列表。不同文件并行处理；同一文件的多处编辑按顺序原子执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文件路径（相对于项目目录）"},
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

func (t *WriteFileTool) Execute(ctx context.Context, args string) *Result {
	type fileItem struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		OldText string `json:"old_text"`
	}
	var params struct {
		Path    string     `json:"path"`
		Content string     `json:"content"`
		OldText string     `json:"old_text"`
		Files   []fileItem `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		var raw map[string]json.RawMessage
		if json.Unmarshal([]byte(args), &raw) == nil {
			if filesRaw, ok := raw["files"]; ok {
				var filesStr string
				if json.Unmarshal(filesRaw, &filesStr) == nil {
					if json.Unmarshal([]byte(filesStr), &params.Files) == nil {
						goto parsed
					}
				}
			}
		}
		return ErrorResult("参数解析失败: " + err.Error())
	}
parsed:

	if len(params.Files) > 0 {
		pathOrder := []string{}
		fileOps := map[string][]writeOp{}
		for _, f := range params.Files {
			if _, exists := fileOps[f.Path]; !exists {
				pathOrder = append(pathOrder, f.Path)
			}
			fileOps[f.Path] = append(fileOps[f.Path], writeOp{f.Content, f.OldText})
		}

		n := len(pathOrder)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, path := range pathOrder {
			wg.Add(1)
			go func(idx int, filePath string, ops []writeOp) {
				defer wg.Done()
				res := processFileOps(ctx, filePath, ops)
				if res.IsError {
					results[idx] = fmt.Sprintf("✗ %s — %s", filePath, res.Content)
				} else {
					results[idx] = fmt.Sprintf("✓ %s — %s", filePath, res.Content)
				}
			}(i, path, fileOps[path])
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
	fullPath, err := safePath(ctx, params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	if params.OldText != "" {
		edits := []files.EditItem{{OldText: params.OldText, NewText: params.Content}}
		cnt, err := files.ApplyFileEdits(fullPath, edits)
		if err != nil {
			return ErrorResult(err.Error())
		}
		msg := "文件已编辑: " + params.Path
		if cnt > 1 {
			msg = fmt.Sprintf("文件已编辑（%d 处）: %s", cnt, params.Path)
		}
		return successWithDiag(ctx, fullPath, msg)
	}
	if err := files.WriteSingleFile(fullPath, params.Content); err != nil {
		return ErrorResult(err.Error())
	}
	return successWithDiag(ctx, fullPath, "文件已写入: "+params.Path)
}

// successWithDiag 返回成功结果并追加诊断信息（如有）
func successWithDiag(ctx context.Context, fullPath, msg string) *Result {
	if diag := files.DiagnoseFile(ctx, fullPath); diag != "" {
		return SuccessResult(msg + "\n" + diag)
	}
	return SuccessResult(msg)
}

// processFileOps 处理单个文件的操作列表
func processFileOps(ctx context.Context, path string, ops []writeOp) *Result {
	if path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := safePath(ctx, path)
	if err != nil {
		return ErrorResult(err.Error())
	}

	if len(ops) == 1 {
		if ops[0].OldText == "" {
			if err := files.WriteSingleFile(fullPath, ops[0].Content); err != nil {
				return ErrorResult(err.Error())
			}
			return successWithDiag(ctx, fullPath, "文件已写入: "+path)
		}
		edits := []files.EditItem{{OldText: ops[0].OldText, NewText: ops[0].Content}}
		cnt, err := files.ApplyFileEdits(fullPath, edits)
		if err != nil {
			return ErrorResult(err.Error())
		}
		if cnt > 1 {
			return successWithDiag(ctx, fullPath, fmt.Sprintf("文件已编辑（%d 处）: %s", cnt, path))
		}
		return successWithDiag(ctx, fullPath, "文件已编辑: "+path)
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
		return successWithDiag(ctx, fullPath, fmt.Sprintf("文件已编辑（%d 处）: %s", cnt, path))
	}

	// 混合操作：按顺序处理
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
		return successWithDiag(ctx, fullPath, fmt.Sprintf("文件已更新（%d 处编辑）: %s", editCount, path))
	}
	return successWithDiag(ctx, fullPath, "文件已写入: "+path)
}

// ── DeleteFileTool 删除文件或空目录（支持批量）─────────────
type DeleteFileTool struct{}

func (t *DeleteFileTool) Name() string { return "delete_files" }

func (t *DeleteFileTool) Description() string {
	return "删除项目目录下的文件或空目录，并行执行。仅允许删除文件和空目录，不允许递归删除非空目录。此工具操作项目代码目录，删除工作区文档请使用 delete_work_docs。"
}

func (t *DeleteFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要删除的文件/空目录路径列表，并行执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文件或空目录路径（相对于项目目录）"}
					},
					"required": ["path"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *DeleteFileTool) Execute(ctx context.Context, args string) *Result {
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
				fullPath, err := safePath(ctx, path)
				if err != nil {
					results[idx] = fmt.Sprintf("✗ %s — %s", path, err.Error())
					return
				}
				if err := files.DeleteSingleFile(fullPath); err != nil {
					results[idx] = fmt.Sprintf("✗ %s — %s", path, err.Error())
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
		header := fmt.Sprintf("批量操作结果 (%d 项，%d 成功 / %d 失败)：", n, successCount, failCount)
		output := header + "\n" + strings.Join(results, "\n")
		if failCount == n {
			return ErrorResult(output)
		}
		return SuccessResult(output)
	}

	if params.Path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := safePath(ctx, params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	if err := files.DeleteSingleFile(fullPath); err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult("已删除: " + params.Path)
}

// ── ListFilesTool 列出目录内容（支持批量）────────────────

type ListFilesTool struct{}

func (t *ListFilesTool) Name() string { return "list_files" }

func (t *ListFilesTool) Description() string {
	return "列出项目目录下的文件和子目录，支持 glob 模式匹配和递归遍历。注意：此工具操作项目代码目录，查看工作区文档请使用 list_work_docs。"
}

func (t *ListFilesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"dirs": {
				"type": "array",
				"description": "要列出的目录配置列表，并行执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "目录路径（相对于项目目录，默认根目录）"},
						"pattern": {"type": "string", "description": "glob 匹配模式（如 *.go），可选"},
						"recursive": {"type": "boolean", "description": "是否递归遍历子目录，默认 false"}
					}
				}
			}
		},
		"required": ["dirs"]
	}`)
}

func (t *ListFilesTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Path      string `json:"path"`
		Pattern   string `json:"pattern"`
		Recursive bool   `json:"recursive"`
		Dirs      []struct {
			Path      string `json:"path"`
			Pattern   string `json:"pattern"`
			Recursive bool   `json:"recursive"`
		} `json:"dirs"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	if len(params.Dirs) > 0 {
		n := len(params.Dirs)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, d := range params.Dirs {
			wg.Add(1)
			go func(idx int, path, pattern string, recursive bool) {
				defer wg.Done()
				if path == "" {
					path = "."
				}
				fullPath, err := safePath(ctx, path)
				label := path
				if err != nil {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s（错误）===\n%s", idx+1, n, label, err.Error())
					return
				}
				content, err := files.ListSingleDir(fullPath, pattern, recursive)
				if err != nil {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s（错误）===\n%s", idx+1, n, label, err.Error())
				} else {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s ===\n%s", idx+1, n, label, content)
				}
			}(i, d.Path, d.Pattern, d.Recursive)
		}
		wg.Wait()
		return SuccessResult(strings.Join(results, "\n\n"))
	}

	path := params.Path
	if path == "" {
		path = "."
	}
	fullPath, err := safePath(ctx, path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	content, err := files.ListSingleDir(fullPath, params.Pattern, params.Recursive)
	if err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult(content)
}

// ── SearchFilesTool 搜索文件内容（支持批量）──────────────

type SearchFilesTool struct{}

func (t *SearchFilesTool) Name() string { return "search_files" }

func (t *SearchFilesTool) Description() string {
	return "在项目目录中搜索文件内容，类似 grep，并行执行，返回匹配行及行号。注意：此工具操作项目代码目录，工作区文档由系统自动加载到上下文。"
}

func (t *SearchFilesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"queries": {
				"type": "array",
				"description": "搜索配置列表，并行执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "搜索目录（相对于项目目录，默认项目根目录）"},
						"pattern": {"type": "string", "description": "搜索关键词（子串匹配）"},
						"file_pattern": {"type": "string", "description": "文件名 glob 过滤（如 *.go），可选"}
					},
					"required": ["pattern"]
				}
			}
		},
		"required": ["queries"]
	}`)
}

func (t *SearchFilesTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Path        string `json:"path"`
		Pattern     string `json:"pattern"`
		FilePattern string `json:"file_pattern"`
		Queries     []struct {
			Path        string `json:"path"`
			Pattern     string `json:"pattern"`
			FilePattern string `json:"file_pattern"`
		} `json:"queries"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	if len(params.Queries) > 0 {
		n := len(params.Queries)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, q := range params.Queries {
			wg.Add(1)
			go func(idx int, path, pattern, filePattern string) {
				defer wg.Done()
				if pattern == "" {
					results[idx] = fmt.Sprintf("=== [%d/%d] 搜索: %q（错误）===\n搜索关键词不能为空", idx+1, n, pattern)
					return
				}
				if path == "" {
					path = "."
				}
				fullPath, err := safePath(ctx, path)
				if err != nil {
					results[idx] = fmt.Sprintf("=== [%d/%d] 搜索: %q（错误）===\n%s", idx+1, n, pattern, err.Error())
					return
				}
				content, err := files.SearchSinglePattern(fullPath, pattern, filePattern)
				if err != nil {
					results[idx] = fmt.Sprintf("=== [%d/%d] 搜索: %q（错误）===\n%s", idx+1, n, pattern, err.Error())
				} else {
					results[idx] = fmt.Sprintf("=== [%d/%d] 搜索: %q ===\n%s", idx+1, n, pattern, content)
				}
			}(i, q.Path, q.Pattern, q.FilePattern)
		}
		wg.Wait()
		return SuccessResult(strings.Join(results, "\n\n"))
	}

	if params.Pattern == "" {
		return ErrorResult("搜索关键词不能为空")
	}
	path := params.Path
	if path == "" {
		path = "."
	}
	fullPath, err := safePath(ctx, path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	content, err := files.SearchSinglePattern(fullPath, params.Pattern, params.FilePattern)
	if err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult(content)
}

// ── AnalysisFileTool 文件结构分析（支持批量）─────────────

type AnalysisFileTool struct{}

func (t *AnalysisFileTool) Name() string { return "analysis_files" }

func (t *AnalysisFileTool) Description() string {
	return "分析文件结构，输出函数/类型定义、导入、注释等摘要信息（含行号），帮助理解文件内容而无需读取全文。支持代码文件（Go/Python/JS/TS/Java/Rust/C/C++ 等）、文档文件（Markdown）和配置文件（YAML/JSON/TOML）。各文件并行分析。"
}

func (t *AnalysisFileTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要分析的文件列表，并行执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文件路径（相对于项目目录）"}
					},
					"required": ["path"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *AnalysisFileTool) Execute(ctx context.Context, args string) *Result {
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
				fullPath, err := safePath(ctx, path)
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
	fullPath, err := safePath(ctx, params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	content, err := files.AnalyzeSingleFile(fullPath)
	if err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult(content)
}
