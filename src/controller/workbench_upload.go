package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent_flow/src/common"
	"agent_flow/src/service"
)

// UploadFile 工作台文件上传（图片/附件）
// POST /admin/workbench/upload
func (c *WorkbenchCtrl) UploadFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		common.JSONError(w, http.StatusBadRequest, "文件过大或格式错误")
		return
	}

	wsID, _ := strconv.ParseUint(r.FormValue("workspace_id"), 10, 64)
	if wsID == 0 {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 不能为空")
		return
	}

	ws, err := c.WorkspaceService.GetByID(uint(wsID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "未找到上传文件")
		return
	}
	defer file.Close()

	// 检测 MIME 类型
	mediaType := detectMediaType(header.Filename)
	if mediaType == "" {
		common.JSONError(w, http.StatusBadRequest, "不支持的文件类型")
		return
	}

	// 构建保存路径
	workDir := c.WorkspaceService.GetWorkDir(ws.UUID)
	uploadDir := filepath.Join(workDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "创建上传目录失败")
		return
	}

	// 生成文件名：时间戳_原始文件名
	ext := filepath.Ext(header.Filename)
	baseName := strings.TrimSuffix(header.Filename, ext)
	if len(baseName) > 50 {
		baseName = baseName[:50]
	}
	filename := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102150405"), baseName, ext)
	savePath := filepath.Join(uploadDir, filename)

	// 保存文件
	dst, err := os.Create(savePath)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "保存文件失败")
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "写入文件失败")
		return
	}

	// 返回附件信息（路径相对于工作区目录）
	common.JSONSuccess(w, map[string]interface{}{
		"path":       "uploads/" + filename,
		"name":       header.Filename,
		"media_type": mediaType,
		"size":       written,
	})
}

// ListUploads 获取工作区附件列表（uploads/ + screenshots/）
// GET /admin/workbench/uploads/list?workspace_id=X
func (c *WorkbenchCtrl) ListUploads(w http.ResponseWriter, r *http.Request) {
	wsIDStr := r.URL.Query().Get("workspace_id")
	if wsIDStr == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 不能为空")
		return
	}

	wsID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工作区 ID")
		return
	}

	ws, err := c.WorkspaceService.GetByID(uint(wsID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	workDir := c.WorkspaceService.GetWorkDir(ws.UUID)

	type uploadFile struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		Size      int64  `json:"size"`
		SizeText  string `json:"size_text"`
		ModTime   string `json:"mod_time"`
		MediaType string `json:"media_type"`
		Scope     string `json:"scope"`
	}

	var files []uploadFile
	for _, scope := range []string{"uploads", "screenshots"} {
		dir := filepath.Join(workDir, scope)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			mediaType := detectMediaType(e.Name())
			if mediaType == "" {
				mediaType = "application/octet-stream"
			}
			files = append(files, uploadFile{
				Name:      e.Name(),
				Path:      scope + "/" + e.Name(),
				Size:      info.Size(),
				SizeText:  formatSizeText(info.Size()),
				ModTime:   info.ModTime().Format("2006-01-02 15:04"),
				MediaType: mediaType,
				Scope:     scope,
			})
		}
	}

	common.JSONSuccess(w, map[string]interface{}{
		"files": files,
	})
}

// SaveUpload 保存附件文本内容（编辑后保存）
// POST /admin/workbench/uploads/save
func (c *WorkbenchCtrl) SaveUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID uint   `json:"workspace_id"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSONError(w, http.StatusBadRequest, "参数解析失败")
		return
	}
	if req.WorkspaceID == 0 {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 不能为空")
		return
	}
	if req.Path == "" {
		common.JSONError(w, http.StatusBadRequest, "path 不能为空")
		return
	}

	cleaned := filepath.Clean(req.Path)
	if !strings.HasPrefix(cleaned, "uploads"+string(filepath.Separator)) && !strings.HasPrefix(cleaned, "uploads/") {
		common.JSONError(w, http.StatusBadRequest, "只允许编辑 uploads/ 目录下的文件")
		return
	}

	ws, err := c.WorkspaceService.GetByID(req.WorkspaceID)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	workDir := c.WorkspaceService.GetWorkDir(ws.UUID)
	fullPath := filepath.Join(workDir, cleaned)

	absWork, _ := filepath.Abs(workDir)
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absWork) {
		common.JSONError(w, http.StatusBadRequest, "非法路径")
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	common.JSONSuccess(w, nil)
}

// ServeUpload 提供上传文件的访问（用于前端显示图片）
// GET /admin/workbench/uploads?workspace_id=X&path=uploads/xxx.png
func (c *WorkbenchCtrl) ServeUpload(w http.ResponseWriter, r *http.Request) {
	wsID, _ := strconv.ParseUint(r.URL.Query().Get("workspace_id"), 10, 64)
	if wsID == 0 {
		http.Error(w, "workspace_id 不能为空", http.StatusBadRequest)
		return
	}

	ws, err := c.WorkspaceService.GetByID(uint(wsID))
	if err != nil {
		http.Error(w, "工作区不存在", http.StatusNotFound)
		return
	}

	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		http.Error(w, "path 不能为空", http.StatusBadRequest)
		return
	}

	// 安全检查：防止路径遍历
	cleanPath := filepath.Clean(relPath)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "非法路径", http.StatusForbidden)
		return
	}

	workDir := c.WorkspaceService.GetWorkDir(ws.UUID)
	fullPath := filepath.Join(workDir, cleanPath)

	// 确认文件在工作区目录内
	absWork, _ := filepath.Abs(workDir)
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absWork) {
		http.Error(w, "非法路径", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, fullPath)
}

// detectMediaType 根据文件扩展名检测 MIME 类型
func detectMediaType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	types := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".bmp":  "image/bmp",
		".pdf":  "application/pdf",
		".txt":  "text/plain",
		".md":   "text/markdown",
	}
	return types[ext]
}

// EnhancePrompt 提示词增强
// POST /admin/workbench/enhance-prompt
func (c *WorkbenchCtrl) EnhancePrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message     string `json:"message"`
		WorkspaceID uint   `json:"workspace_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSONError(w, http.StatusBadRequest, "参数解析失败")
		return
	}
	if req.Message == "" {
		common.JSONError(w, http.StatusBadRequest, "message 不能为空")
		return
	}

	if c.PromptEnhanceService == nil {
		common.JSONError(w, http.StatusServiceUnavailable, "提示词增强服务未配置")
		return
	}

	enhanced, err := c.PromptEnhanceService.Enhance(r.Context(), req.Message)
	if err != nil {
		if errors.Is(err, service.ErrPromptTruncated) && enhanced != "" {
			common.JSONSuccess(w, map[string]string{
				"enhanced": enhanced,
				"warning":  err.Error(),
			})
			return
		}
		common.JSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	common.JSONSuccess(w, map[string]string{"enhanced": enhanced})
}
