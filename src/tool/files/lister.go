package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListSingleDir 列出目录内容，支持 glob 模式和递归遍历
func ListSingleDir(fullPath, pattern string, recursive bool) (string, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("路径不存在: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("路径不是目录")
	}

	var results []string

	if recursive {
		err = filepath.Walk(fullPath, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(fullPath, p)
			if rel == "." {
				return nil
			}
			if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			if pattern != "" {
				matched, _ := filepath.Match(pattern, info.Name())
				if !matched {
					return nil
				}
			}
			entry := rel
			if info.IsDir() {
				entry += "/"
			}
			results = append(results, entry)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("遍历目录失败: %w", err)
		}
	} else {
		entries, readErr := os.ReadDir(fullPath)
		if readErr != nil {
			return "", fmt.Errorf("读取目录失败: %w", readErr)
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}
			if pattern != "" {
				matched, _ := filepath.Match(pattern, name)
				if !matched {
					continue
				}
			}
			entry := name
			if e.IsDir() {
				entry += "/"
			}
			results = append(results, entry)
		}
	}

	if len(results) == 0 {
		return "目录为空或无匹配文件", nil
	}

	const maxEntries = 500
	truncated := false
	if len(results) > maxEntries {
		results = results[:maxEntries]
		truncated = true
	}

	output := strings.Join(results, "\n")
	if truncated {
		output += fmt.Sprintf("\n... (结果已截断，共超过 %d 条)", maxEntries)
	}

	return output, nil
}
