package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EditItem 编辑操作项
type EditItem struct {
	OldText string
	NewText string
}

// WriteSingleFile 写入文件内容，自动创建父目录
func WriteSingleFile(fullPath, content string) error {
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}

// ApplyFileEdits 对文件应用多处搜索替换编辑（原子读写）
func ApplyFileEdits(fullPath string, edits []EditItem) (int, error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return 0, fmt.Errorf("读取文件失败: %w", err)
	}

	content := string(data)
	for i, edit := range edits {
		count := strings.Count(content, edit.OldText)
		if count == 0 {
			return 0, fmt.Errorf("第%d处编辑失败：未找到匹配的文本", i+1)
		}
		if count > 1 {
			return 0, fmt.Errorf("第%d处编辑失败：找到 %d 处匹配，old_text 必须唯一匹配", i+1, count)
		}
		content = strings.Replace(content, edit.OldText, edit.NewText, 1)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return 0, fmt.Errorf("写入文件失败: %w", err)
	}
	return len(edits), nil
}
