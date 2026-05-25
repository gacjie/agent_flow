package files

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SearchInFile 在单个文件中搜索关键词，返回 "行号:内容" 列表
func SearchInFile(path, pattern string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	lowerPattern := strings.ToLower(pattern)

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), lowerPattern) {
			display := line
			if len(display) > 200 {
				display = display[:200] + "..."
			}
			results = append(results, fmt.Sprintf("%d:%s", lineNum, display))
		}
	}
	return results
}

// SearchSinglePattern 在目录中搜索文件内容
func SearchSinglePattern(fullPath, pattern, filePattern string) (string, error) {
	var matches []string
	const maxMatches = 200

	_ = filepath.Walk(fullPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		if info.Size() > 1024*1024 {
			return nil
		}
		if filePattern != "" {
			matched, _ := filepath.Match(filePattern, info.Name())
			if !matched {
				return nil
			}
		}
		fileMatches := SearchInFile(p, pattern)
		if len(fileMatches) > 0 {
			rel, _ := filepath.Rel(fullPath, p)
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
		return "未找到匹配内容", nil
	}

	output := strings.Join(matches, "\n")
	if len(matches) >= maxMatches {
		output += fmt.Sprintf("\n... (结果已截断，最多显示 %d 条)", maxMatches)
	}

	return output, nil
}
