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

// workspacePath 将相对路径解析为工作区目录下的绝对路径并校验安全性
func workspacePath(ctx context.Context, path string) (string, error) {
	wc := GetWorkContext(ctx)
	if wc == nil || wc.WorkspaceDir == "" {
		return "", fmt.Errorf("工作区目录未设置")
	}

	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("不允许使用绝对路径: %s", path)
	}
	fullPath := filepath.Join(wc.WorkspaceDir, cleaned)

	// 校验路径在工作区范围内
	if !isSubPath(fullPath, wc.WorkspaceDir) {
		return "", fmt.Errorf("路径超出工作区范围: %s", path)
	}
	return fullPath, nil
}

func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%dB", bytes)
	}
	return fmt.Sprintf("%.1fKB", float64(bytes)/1024)
}

// ── ReadWorkDocTool 读取工作区文档（支持批量）──────────────

type ReadWorkDocTool struct{}

func (t *ReadWorkDocTool) Name() string { return "read_work_docs" }

func (t *ReadWorkDocTool) Description() string {
	return "读取工作区目录中的文档内容，并行读取。路径相对于工作区目录，如 specs/requirement.md、tasks/backend-api.md。工作区目录独立于项目代码目录。"
}

func (t *ReadWorkDocTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要读取的文档列表，并行读取",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文档路径（相对于工作区目录），如 specs/requirement.md、tasks/backend-api.md"}
					},
					"required": ["path"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *ReadWorkDocTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Path  string `json:"path"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	// 批量模式
	if len(params.Files) > 0 {
		n := len(params.Files)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, f := range params.Files {
			wg.Add(1)
			go func(idx int, path string) {
				defer wg.Done()
				res := readSingleWorkDocByPath(ctx, path)
				if res.IsError {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s（错误）===\n%s", idx+1, n, path, res.Content)
				} else {
					results[idx] = fmt.Sprintf("=== [%d/%d] %s ===\n%s", idx+1, n, path, res.Content)
				}
			}(i, f.Path)
		}
		wg.Wait()
		return SuccessResult(strings.Join(results, "\n\n"))
	}

	// 单文档模式
	return readSingleWorkDocByPath(ctx, params.Path)
}

func readSingleWorkDocByPath(ctx context.Context, path string) *Result {
	if path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := workspacePath(ctx, path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ErrorResult("读取文件失败: " + err.Error())
	}
	return SuccessResult(string(data))
}

// ── WriteWorkDocTool 创建/编辑工作区文档（支持批量）───────

type WriteWorkDocTool struct{}

func (t *WriteWorkDocTool) Name() string { return "write_work_docs" }

func (t *WriteWorkDocTool) Description() string {
	return "在工作区目录创建或编辑文档，自动创建不存在的子目录。路径相对于工作区目录，如 specs/requirement.md、tasks/backend-api.md。specs/ 下的文档会被系统自动加载到上下文中，tasks/ 下的文档由任务节点的 task_doc 字段引用、按需加载。支持 write（覆盖，默认）和 append（追加）两种模式。\n\n注意：单次调用的内容不宜过长（建议单个文档不超过 3000 字），超长内容请拆分为多个文档或多次调用。"
}

func (t *WriteWorkDocTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "文档操作列表，顺序写入",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文档路径（相对于工作区目录），如 specs/requirement.md、tasks/backend-api.md"},
						"content": {"type": "string", "description": "文档内容（Markdown 格式）"},
						"mode": {"type": "string", "enum": ["write", "append"], "description": "写入模式：write（默认，覆盖）/ append（追加到文件末尾）"}
					},
					"required": ["path", "content"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *WriteWorkDocTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Mode    string `json:"mode"`
		Files   []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
			Mode    string `json:"mode"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	// 批量模式（顺序执行）
	if len(params.Files) > 0 {
		results := []string{}
		for i, f := range params.Files {
			if f.Path == "" {
				return ErrorResult(fmt.Sprintf("第%d个文档 path 不能为空", i+1))
			}
			if f.Content == "" {
				return ErrorResult(fmt.Sprintf("第%d个文档 content 不能为空", i+1))
			}
			mode := f.Mode
			if mode == "" {
				mode = "write"
			}
			res := writeSingleWorkDocByPath(ctx, f.Path, f.Content, mode)
			if res.IsError {
				results = append(results, fmt.Sprintf("✗ %s — 错误: %s", f.Path, res.Content))
			} else {
				results = append(results, fmt.Sprintf("✓ %s — %s", f.Path, res.Content))
			}
		}
		n := len(params.Files)
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

	// 单文档模式
	if params.Path == "" {
		return ErrorResult("path 不能为空")
	}
	if params.Content == "" {
		return ErrorResult("content 不能为空")
	}
	mode := params.Mode
	if mode == "" {
		mode = "write"
	}
	return writeSingleWorkDocByPath(ctx, params.Path, params.Content, mode)
}

func writeSingleWorkDocByPath(ctx context.Context, path, content, mode string) *Result {
	fullPath, err := workspacePath(ctx, path)
	if err != nil {
		return ErrorResult(err.Error())
	}

	// 自动创建父目录
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ErrorResult("创建目录失败: " + err.Error())
	}

	if mode == "append" {
		existing, readErr := os.ReadFile(fullPath)
		if readErr == nil && len(existing) > 0 {
			content = string(existing) + "\n\n" + content
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return ErrorResult("写入文件失败: " + err.Error())
		}
		if readErr == nil {
			return SuccessResult(fmt.Sprintf("文档已追加: %s", path))
		}
		return SuccessResult(fmt.Sprintf("文档已创建: %s", path))
	}

	// write 模式
	_, statErr := os.Stat(fullPath)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return ErrorResult("写入文件失败: " + err.Error())
	}
	if statErr == nil {
		return SuccessResult(fmt.Sprintf("文档已更新: %s", path))
	}
	return SuccessResult(fmt.Sprintf("文档已创建: %s", path))
}

// ── DeleteWorkDocTool 删除工作区文档（支持批量）──────────

type DeleteWorkDocTool struct{}

func (t *DeleteWorkDocTool) Name() string { return "delete_work_docs" }

func (t *DeleteWorkDocTool) Description() string {
	return "删除工作区目录中指定的文档，并行删除。路径相对于工作区目录，如 specs/requirement.md、tasks/backend-api.md。仅允许删除文件，不允许删除目录。"
}

func (t *DeleteWorkDocTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要删除的文档路径列表，并行删除",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文档路径（相对于工作区目录），如 specs/requirement.md、tasks/backend-api.md"}
					},
					"required": ["path"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *DeleteWorkDocTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Path  string `json:"path"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	// 批量模式
	if len(params.Files) > 0 {
		n := len(params.Files)
		results := make([]string, n)
		var wg sync.WaitGroup
		for i, f := range params.Files {
			wg.Add(1)
			go func(idx int, path string) {
				defer wg.Done()
				res := deleteSingleWorkDocByPath(ctx, path)
				if res.IsError {
					results[idx] = fmt.Sprintf("✗ %s — %s", path, res.Content)
				} else {
					results[idx] = fmt.Sprintf("✓ %s — %s", path, res.Content)
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

	// 单文档模式
	return deleteSingleWorkDocByPath(ctx, params.Path)
}

func deleteSingleWorkDocByPath(ctx context.Context, path string) *Result {
	if path == "" {
		return ErrorResult("path 不能为空")
	}
	fullPath, err := workspacePath(ctx, path)
	if err != nil {
		return ErrorResult(err.Error())
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return ErrorResult("文件不存在: " + err.Error())
	}
	if info.IsDir() {
		return ErrorResult("不允许删除目录: " + path)
	}
	if err := os.Remove(fullPath); err != nil {
		return ErrorResult("删除失败: " + err.Error())
	}
	return SuccessResult("已删除: " + path)
}

// ── ListWorkDocsTool 列出工作区文档 ───────────────────────

type ListWorkDocsTool struct{}

func (t *ListWorkDocsTool) Name() string { return "list_work_docs" }

func (t *ListWorkDocsTool) Description() string {
	return "列出工作区目录中的文档。默认显示 specs/ 和 tasks/ 两个目录下的全部文件，可通过 scope 参数指定只显示某个目录。工作区目录独立于项目代码目录，list_files 无法查看工作区文档，需用此工具查看。"
}

func (t *ListWorkDocsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"scope": {
				"type": "string",
				"enum": ["specs", "tasks"],
				"description": "可选，指定只列出 specs 或 tasks 目录，不传则显示两者"
			}
		}
	}`)
}

func (t *ListWorkDocsTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Scope string `json:"scope"`
	}
	if args != "" {
		json.Unmarshal([]byte(args), &params)
	}

	wc := GetWorkContext(ctx)
	if wc == nil || wc.WorkspaceDir == "" {
		return ErrorResult("工作区目录未设置")
	}

	scopes := []string{"specs", "tasks"}
	if params.Scope == "specs" || params.Scope == "tasks" {
		scopes = []string{params.Scope}
	}

	var sb strings.Builder
	totalFiles := 0

	for _, scope := range scopes {
		dir := filepath.Join(wc.WorkspaceDir, scope)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				sb.WriteString(fmt.Sprintf("%s/  （目录不存在）\n", scope))
				continue
			}
			sb.WriteString(fmt.Sprintf("%s/  （读取失败: %s）\n", scope, err.Error()))
			continue
		}

		var files []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			files = append(files, fmt.Sprintf("  %s/%s  (%s)", scope, e.Name(), formatSize(info.Size())))
		}

		if len(files) == 0 {
			sb.WriteString(fmt.Sprintf("%s/  （空）\n", scope))
		} else {
			sb.WriteString(fmt.Sprintf("%s/ (%d 个文件):\n%s\n", scope, len(files), strings.Join(files, "\n")))
			totalFiles += len(files)
		}
	}

	if totalFiles == 0 {
		return SuccessResult("工作区暂无文档\n" + sb.String())
	}
	return SuccessResult(sb.String())
}

// ── AnalysisWorkDocTool 分析工作区文档结构（支持批量）─────

type AnalysisWorkDocTool struct{}

func (t *AnalysisWorkDocTool) Name() string { return "analysis_work_docs" }

func (t *AnalysisWorkDocTool) Description() string {
	return "分析工作区目录中的文件结构，输出标题/代码块/链接等摘要信息（含行号），帮助理解文件内容而无需读取全文。路径相对于工作区目录，如 specs/requirement.md、tasks/backend-api.md。各文件并行分析。"
}

func (t *AnalysisWorkDocTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"files": {
				"type": "array",
				"description": "要分析的文档列表，并行执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文档路径（相对于工作区目录），如 specs/requirement.md、tasks/backend-api.md"}
					},
					"required": ["path"]
				}
			}
		},
		"required": ["files"]
	}`)
}

func (t *AnalysisWorkDocTool) Execute(ctx context.Context, args string) *Result {
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
				fullPath, err := workspacePath(ctx, path)
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
	fullPath, err := workspacePath(ctx, params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}
	content, err := files.AnalyzeSingleFile(fullPath)
	if err != nil {
		return ErrorResult(err.Error())
	}
	return SuccessResult(content)
}
