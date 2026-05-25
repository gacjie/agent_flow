package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agent_flow/src/tool/files/analyzers"
)

const maxAnalysisFileSize = 1024 * 1024

var extToLang = map[string]string{
	".go": "go", ".py": "python", ".js": "javascript", ".ts": "typescript",
	".jsx": "jsx", ".tsx": "tsx", ".java": "java", ".rs": "rust",
	".c": "c", ".cpp": "cpp", ".h": "c-header", ".hpp": "cpp-header",
	".cs": "csharp", ".rb": "ruby", ".php": "php", ".swift": "swift",
	".kt": "kotlin", ".vue": "vue", ".sh": "shell",
	".md": "markdown",
	".yaml": "yaml", ".yml": "yaml", ".json": "json", ".toml": "toml",
	".html": "html", ".css": "css", ".sql": "sql",
}

var codeExts = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
	".java": true, ".rs": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true,
	".cs": true, ".rb": true, ".php": true, ".swift": true, ".kt": true, ".vue": true, ".sh": true,
}

var docExts = map[string]bool{".md": true}

var configExts = map[string]bool{
	".yaml": true, ".yml": true, ".json": true, ".toml": true,
}

var webExts = map[string]bool{".html": true, ".css": true, ".sql": true}

func humanSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func AnalyzeSingleFile(fullPath string) (string, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("文件不存在: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("路径是目录，不是文件")
	}
	if info.Size() > maxAnalysisFileSize {
		return "", fmt.Errorf("文件过大（%s），超过分析上限 1MB", humanSize(info.Size()))
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}

	if len(data) > 512 {
		for _, b := range data[:512] {
			if b == 0 {
				return "", fmt.Errorf("二进制文件，跳过分析")
			}
		}
	}

	content := string(data)
	ext := strings.ToLower(filepath.Ext(fullPath))
	lang := extToLang[ext]
	if lang == "" {
		return "", fmt.Errorf("不支持的文件类型: %s", ext)
	}

	lineCount := strings.Count(content, "\n") + 1
	header := fmt.Sprintf("[%s] %d 行 | %s", lang, lineCount, humanSize(info.Size()))

	var analysis string
	switch {
	case codeExts[ext]:
		analysis = analyzers.AnalyzeCode(content, lang)
	case docExts[ext]:
		analysis = analyzers.AnalyzeMarkdown(content)
	case configExts[ext]:
		analysis = analyzers.AnalyzeConfig(content, lang)
	case webExts[ext]:
		analysis = analyzers.AnalyzeWeb(content, lang)
	default:
		return "", fmt.Errorf("不支持的文件类型: %s", ext)
	}

	return header + "\n" + analysis, nil
}
