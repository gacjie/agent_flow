package tool

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MemoryUpdateItem 记忆更新条目（old_memory/new_memory 差异模式）
type MemoryUpdateItem struct {
	OldMemory string `json:"old_memory"`
	NewMemory string `json:"new_memory"`
}

// GetMemoryPath 获取智能体记忆文件路径
func GetMemoryPath(contextRoot, agentName string) string {
	return filepath.Join(contextRoot, "agents", agentName, "memory.md")
}

var multiNewlines = regexp.MustCompile(`\n{3,}`)

// ApplyMemoryUpdates 执行记忆更新操作（追加/修改/删除）
// 所有操作在内存中完成后一次性写回文件
func ApplyMemoryUpdates(memoryPath string, updates []MemoryUpdateItem) (results []string, hasError bool) {
	existing := ""
	if data, err := os.ReadFile(memoryPath); err == nil {
		existing = string(data)
	}

	content := existing
	for i, u := range updates {
		oldEmpty := strings.TrimSpace(u.OldMemory) == ""
		newEmpty := strings.TrimSpace(u.NewMemory) == ""

		switch {
		case oldEmpty && newEmpty:
			// 跳过
			continue

		case oldEmpty && !newEmpty:
			// 追加
			if content != "" && !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += "\n" + u.NewMemory + "\n"
			results = append(results, fmt.Sprintf("✓ 第%d条：已追加", i+1))

		case !oldEmpty && !newEmpty:
			// 修改
			if strings.Contains(content, u.OldMemory) {
				content = strings.Replace(content, u.OldMemory, u.NewMemory, 1)
				results = append(results, fmt.Sprintf("✓ 第%d条：已修改", i+1))
			} else {
				results = append(results, fmt.Sprintf("✗ 第%d条：未找到匹配内容，修改失败", i+1))
				hasError = true
			}

		case !oldEmpty && newEmpty:
			// 删除
			if strings.Contains(content, u.OldMemory) {
				content = strings.Replace(content, u.OldMemory, "", 1)
				results = append(results, fmt.Sprintf("✓ 第%d条：已删除", i+1))
			} else {
				results = append(results, fmt.Sprintf("✗ 第%d条：未找到匹配内容，删除失败", i+1))
				hasError = true
			}
		}
	}

	// 清理多余空行
	content = multiNewlines.ReplaceAllString(content, "\n\n")
	content = strings.TrimSpace(content)
	if content != "" {
		content += "\n"
	}

	// 写回文件
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0755); err != nil {
		return []string{fmt.Sprintf("✗ 创建目录失败: %s", err.Error())}, true
	}
	if err := os.WriteFile(memoryPath, []byte(content), 0644); err != nil {
		return []string{fmt.Sprintf("✗ 写入文件失败: %s", err.Error())}, true
	}

	return results, hasError
}
