package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"agent_flow/src/tool/files"
)

// DiagnoseFilesTool 代码语法诊断工具
type DiagnoseFilesTool struct{}

func (t *DiagnoseFilesTool) Name() string { return "diagnose_files" }

func (t *DiagnoseFilesTool) Description() string {
	return "对代码文件执行语法诊断，检测语法错误（括号不匹配、缺少分号、无效表达式等）。支持 Go/Python/JavaScript/TypeScript/Java/Rust/C/C++ 等常用语言。不需要安装对应语言的编译器或 linter，内置语法解析引擎。"
}

func (t *DiagnoseFilesTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "单个文件路径（相对于项目目录）"},
			"files": {
				"type": "array",
				"description": "批量诊断的文件路径列表，并行执行",
				"items": {
					"type": "object",
					"properties": {
						"path": {"type": "string", "description": "文件路径"}
					},
					"required": ["path"]
				}
			}
		}
	}`)
}

func (t *DiagnoseFilesTool) Execute(ctx context.Context, args string) *Result {
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
		return t.diagnoseBatch(ctx, params.Files)
	}

	if params.Path == "" {
		return ErrorResult("path 不能为空")
	}

	fullPath, err := safePath(ctx, params.Path)
	if err != nil {
		return ErrorResult(err.Error())
	}

	output := files.DiagnoseFileDetailed(ctx, fullPath)
	return SuccessResult(output)
}

func (t *DiagnoseFilesTool) diagnoseBatch(ctx context.Context, items []struct{ Path string `json:"path"` }) *Result {
	n := len(items)
	results := make([]string, n)
	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			fullPath, err := safePath(ctx, path)
			if err != nil {
				results[idx] = fmt.Sprintf("=== [%d/%d] %s ===\n%s", idx+1, n, path, err.Error())
				return
			}
			output := files.DiagnoseFileDetailed(ctx, fullPath)
			results[idx] = fmt.Sprintf("=== [%d/%d] %s ===\n%s", idx+1, n, path, output)
		}(i, item.Path)
	}
	wg.Wait()

	return SuccessResult(strings.Join(results, "\n\n"))
}
