package browsers

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ScreenshotResult 截屏结果
type ScreenshotResult struct {
	Message  string // 文本描述
	IsError  bool   // 是否出错
	Data     string // base64 编码的图片数据（出错时为空）
	FilePath string // 保存的文件路径（相对于工作区）
}

func DoScreenshot(key string, workspaceDir string) ScreenshotResult {
	sess := Mgr.Get(key)
	if sess == nil {
		return ScreenshotResult{Message: "浏览器会话不存在，请先使用 navigate 打开页面", IsError: true}
	}

	buf, err := sess.Page.Screenshot(true, nil)
	if err != nil {
		return ScreenshotResult{Message: "截图失败: " + err.Error(), IsError: true}
	}

	saveDir := "."
	if workspaceDir != "" {
		saveDir = workspaceDir
	}
	screenshotDir := filepath.Join(saveDir, "screenshots")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		return ScreenshotResult{Message: "创建截图目录失败: " + err.Error(), IsError: true}
	}

	filename := fmt.Sprintf("%s.png", time.Now().Format("20060102-150405"))
	savePath := filepath.Join(screenshotDir, filename)
	if err := os.WriteFile(savePath, buf, 0644); err != nil {
		return ScreenshotResult{Message: "保存截图失败: " + err.Error(), IsError: true}
	}

	return ScreenshotResult{
		Message:  fmt.Sprintf("截图已保存: %s（%d 字节）", savePath, len(buf)),
		IsError:  false,
		Data:     base64.StdEncoding.EncodeToString(buf),
		FilePath: filepath.Join("screenshots", filename),
	}
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
