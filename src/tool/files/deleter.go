package files

import (
	"fmt"
	"os"
)

// DeleteSingleFile 删除文件或空目录
func DeleteSingleFile(fullPath string) error {
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("路径不存在: %w", err)
	}

	if info.IsDir() {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return fmt.Errorf("读取目录失败: %w", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("目录非空，不允许递归删除")
		}
	}

	if err := os.Remove(fullPath); err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	return nil
}
