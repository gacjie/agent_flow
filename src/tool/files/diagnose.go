package files

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"agent_flow/src/tool/files/diagnostics"
)

// DiagnoseFile 对文件执行语法诊断，返回格式化的诊断文本
// 不支持的文件类型或无错误时返回空字符串
func DiagnoseFile(ctx context.Context, fullPath string) string {
	ext := strings.ToLower(filepath.Ext(fullPath))
	lang := extToLang[ext]
	if lang == "" {
		return ""
	}
	if !codeExts[ext] {
		return ""
	}

	info, err := os.Stat(fullPath)
	if err != nil || info.Size() > maxAnalysisFileSize {
		return ""
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	engine := diagnostics.GetEngine()
	result := engine.Diagnose(ctx, content, lang, filepath.Base(fullPath))
	return diagnostics.FormatResultBrief(result)
}

// DiagnoseFileDetailed 详细诊断（用于独立工具）
func DiagnoseFileDetailed(ctx context.Context, fullPath string) string {
	ext := strings.ToLower(filepath.Ext(fullPath))
	if !codeExts[ext] {
		return "该文件类型不支持语法诊断"
	}

	lang := extToLang[ext]
	if lang == "" {
		return "该文件类型不支持语法诊断"
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "文件不存在: " + err.Error()
	}
	if info.Size() > maxAnalysisFileSize {
		return "文件过大，跳过诊断"
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "读取文件失败: " + err.Error()
	}

	engine := diagnostics.GetEngine()
	result := engine.Diagnose(ctx, content, lang, filepath.Base(fullPath))

	if !result.Supported {
		return "该语言暂不支持语法诊断"
	}

	formatted := diagnostics.FormatResult(result)
	if formatted == "" {
		return "语法检查通过，无错误"
	}
	return formatted
}
