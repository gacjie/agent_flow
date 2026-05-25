package files

import (
	"os"
	"strconv"
	"strings"
)

// ReadSingleFile 读取文件内容，支持 offset/limit 按行读取
func ReadSingleFile(fullPath string, offset, limit int) (string, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	content := string(data)

	if offset > 0 || limit > 0 {
		lines := strings.Split(content, "\n")
		start := 0
		if offset > 0 {
			start = offset - 1
		}
		if start >= len(lines) {
			return "", nil
		}
		end := len(lines)
		if limit > 0 && start+limit < end {
			end = start + limit
		}

		var sb strings.Builder
		for i := start; i < end; i++ {
			sb.WriteString(strconv.Itoa(i+1) + "\t" + lines[i] + "\n")
		}
		return sb.String(), nil
	}

	return content, nil
}
