package diagnostics

import (
	"fmt"
	"strings"
)

// FormatResult 将诊断结果格式化为可读文本
func FormatResult(result *DiagResult) string {
	if !result.Supported {
		return ""
	}
	if result.ParseError != "" {
		return fmt.Sprintf("[诊断] 引擎错误: %s", result.ParseError)
	}
	if len(result.Diagnostics) == 0 {
		return ""
	}

	var sb strings.Builder
	errors := 0
	warnings := 0
	for _, d := range result.Diagnostics {
		switch d.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		}
	}

	sb.WriteString(fmt.Sprintf("[诊断] 发现 %d 个语法错误", errors))
	if warnings > 0 {
		sb.WriteString(fmt.Sprintf(", %d 个警告", warnings))
	}
	sb.WriteString(":\n")

	for _, d := range result.Diagnostics {
		loc := fmt.Sprintf("L%d:%d", d.Line, d.Column)
		sb.WriteString(fmt.Sprintf("  %s [%s] %s", loc, d.Severity, d.Message))
		if d.Context != "" {
			sb.WriteString(fmt.Sprintf("\n         | %s", d.Context))
		}
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// FormatResultBrief 简短格式（用于 write_files 追加）
func FormatResultBrief(result *DiagResult) string {
	if !result.Supported || result.ParseError != "" || len(result.Diagnostics) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[诊断] 发现 %d 个语法错误:\n", len(result.Diagnostics)))

	for _, d := range result.Diagnostics {
		sb.WriteString(fmt.Sprintf("  L%d:%d %s\n", d.Line, d.Column, d.Message))
	}

	return strings.TrimRight(sb.String(), "\n")
}
