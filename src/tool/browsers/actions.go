package browsers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func DoScreenshot(key string, workspaceDir string) (string, bool) {
	sess := Mgr.Get(key)
	if sess == nil {
		return "浏览器会话不存在，请先使用 navigate 打开页面", true
	}

	buf, err := sess.Page.Screenshot(true, nil)
	if err != nil {
		return "截图失败: " + err.Error(), true
	}

	saveDir := "."
	if workspaceDir != "" {
		saveDir = workspaceDir
	}
	screenshotDir := filepath.Join(saveDir, "screenshots")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		return "创建截图目录失败: " + err.Error(), true
	}

	filename := fmt.Sprintf("%s.png", time.Now().Format("20060102-150405"))
	savePath := filepath.Join(screenshotDir, filename)
	if err := os.WriteFile(savePath, buf, 0644); err != nil {
		return "保存截图失败: " + err.Error(), true
	}

	return fmt.Sprintf("截图已保存: %s（%d 字节）", savePath, len(buf)), false
}

func AppendConsoleInfo(sb *strings.Builder, sess *Session) {
	sess.Mu.Lock()
	logs := make([]string, len(sess.ConsoleLogs))
	copy(logs, sess.ConsoleLogs)
	sess.Mu.Unlock()
	if len(logs) > 0 {
		sb.WriteString(fmt.Sprintf("\n--- 控制台日志（%d 条）---\n", len(logs)))
		sb.WriteString(strings.Join(logs, "\n"))
	}
}
