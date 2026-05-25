package controller

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"agent_flow/src/common"
)

type fileNode struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	IsDir         bool   `json:"is_dir"`
	Size          int64  `json:"size"`
	SizeText      string `json:"size_text,omitempty"`
	ChildrenCount int    `json:"children_count,omitempty"`
}

var ignoredDirs = map[string]bool{
	"node_modules": true, "vendor": true, ".git": true,
	".idea": true, ".vscode": true, "__pycache__": true,
	".next": true, "dist": true, "build": true,
}

// ListProjectFiles 获取项目文件列表（懒加载目录树）
func (c *WorkbenchCtrl) ListProjectFiles(w http.ResponseWriter, r *http.Request) {
	projectPath, _, ok := c.resolveProjectPath(w, r)
	if !ok {
		return
	}

	relPath := r.URL.Query().Get("path")
	targetDir := projectPath
	if relPath != "" {
		cleaned := filepath.Clean(relPath)
		if strings.Contains(cleaned, "..") || filepath.IsAbs(cleaned) {
			common.JSONError(w, http.StatusBadRequest, "非法路径")
			return
		}
		targetDir = filepath.Join(projectPath, cleaned)
		if !strings.HasPrefix(targetDir, projectPath) {
			common.JSONError(w, http.StatusBadRequest, "路径超出项目范围")
			return
		}
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			common.JSONSuccess(w, map[string]interface{}{"files": []fileNode{}})
			return
		}
		common.JSONError(w, http.StatusInternalServerError, "读取目录失败")
		return
	}

	var dirs, files []fileNode
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			if ignoredDirs[name] {
				continue
			}
			childPath := name
			if relPath != "" {
				childPath = filepath.ToSlash(filepath.Join(relPath, name))
			}
			childCount := 0
			if children, err := os.ReadDir(filepath.Join(targetDir, name)); err == nil {
				childCount = len(children)
			}
			dirs = append(dirs, fileNode{
				Name:          name,
				Path:          childPath,
				IsDir:         true,
				ChildrenCount: childCount,
			})
		} else {
			info, err := e.Info()
			if err != nil {
				continue
			}
			filePath := name
			if relPath != "" {
				filePath = filepath.ToSlash(filepath.Join(relPath, name))
			}
			files = append(files, fileNode{
				Name:     name,
				Path:     filePath,
				IsDir:    false,
				Size:     info.Size(),
				SizeText: formatSizeText(info.Size()),
			})
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	result := append(dirs, files...)
	common.JSONSuccess(w, map[string]interface{}{"files": result})
}

// ReadProjectFile 读取项目文件内容
func (c *WorkbenchCtrl) ReadProjectFile(w http.ResponseWriter, r *http.Request) {
	projectPath, _, ok := c.resolveProjectPath(w, r)
	if !ok {
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		common.JSONError(w, http.StatusBadRequest, "path 不能为空")
		return
	}

	fullPath, ok := c.validateProjectFilePath(w, projectPath, filePath)
	if !ok {
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "文件不存在")
		return
	}
	if info.IsDir() {
		common.JSONError(w, http.StatusBadRequest, "不能读取目录")
		return
	}
	if info.Size() > 1<<20 {
		common.JSONError(w, http.StatusBadRequest, "文件过大（超过 1MB）")
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "读取文件失败")
		return
	}

	common.JSONSuccess(w, map[string]string{
		"path":    filePath,
		"content": string(data),
	})
}

// SaveDoc 保存工作区文档
func (c *WorkbenchCtrl) SaveDoc(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID uint   `json:"workspace_id"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSONError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.WorkspaceID == 0 || req.Path == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 和 path 不能为空")
		return
	}

	ws, err := c.WorkspaceService.GetByID(req.WorkspaceID)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	cleaned := filepath.Clean(req.Path)
	if !strings.HasPrefix(cleaned, "specs/") && !strings.HasPrefix(cleaned, "tasks/") &&
		!strings.HasPrefix(cleaned, "specs"+string(filepath.Separator)) && !strings.HasPrefix(cleaned, "tasks"+string(filepath.Separator)) {
		common.JSONError(w, http.StatusBadRequest, "只允许保存 specs/ 或 tasks/ 目录下的文件")
		return
	}

	workDir := c.WorkspaceService.GetWorkDir(ws.UUID)
	fullPath := filepath.Join(workDir, cleaned)
	if !strings.HasPrefix(fullPath, workDir) {
		common.JSONError(w, http.StatusBadRequest, "非法路径")
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	common.JSONSuccess(w, map[string]string{"path": req.Path})
}

// SaveProjectDoc 保存项目文档
func (c *WorkbenchCtrl) SaveProjectDoc(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID uint   `json:"workspace_id"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSONError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.WorkspaceID == 0 || req.Path == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 和 path 不能为空")
		return
	}

	ws, err := c.WorkspaceService.GetByID(req.WorkspaceID)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}
	if ws.ProjectID == 0 || ws.Project.Path == "" {
		common.JSONError(w, http.StatusBadRequest, "工作区未关联项目")
		return
	}

	cleaned := filepath.Clean(req.Path)
	allowed := false
	if cleaned == "AGENTS.md" || cleaned == "README.md" {
		allowed = true
	} else if strings.HasPrefix(cleaned, "docs/") || strings.HasPrefix(cleaned, "docs"+string(filepath.Separator)) {
		name := filepath.Base(cleaned)
		if strings.HasSuffix(strings.ToLower(name), ".md") && filepath.Dir(cleaned) == "docs" {
			allowed = true
		}
	}
	if !allowed {
		common.JSONError(w, http.StatusBadRequest, "只允许保存 AGENTS.md、README.md 或 docs/ 目录下的 .md 文件")
		return
	}

	projectPath := ws.Project.Path
	fullPath := filepath.Join(projectPath, cleaned)
	if !strings.HasPrefix(fullPath, projectPath) {
		common.JSONError(w, http.StatusBadRequest, "非法路径")
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	common.JSONSuccess(w, map[string]string{"path": req.Path})
}

// SaveProjectFile 保存项目文件
func (c *WorkbenchCtrl) SaveProjectFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID uint   `json:"workspace_id"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSONError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.WorkspaceID == 0 || req.Path == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 和 path 不能为空")
		return
	}

	ws, err := c.WorkspaceService.GetByID(req.WorkspaceID)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}
	if ws.ProjectID == 0 || ws.Project.Path == "" {
		common.JSONError(w, http.StatusBadRequest, "工作区未关联项目")
		return
	}

	projectPath := ws.Project.Path
	fullPath, ok := c.validateProjectFilePath(w, projectPath, req.Path)
	if !ok {
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "文件不存在，不允许创建新文件")
		return
	}
	if info.IsDir() {
		common.JSONError(w, http.StatusBadRequest, "不能写入目录")
		return
	}
	if len(req.Content) > 1<<20 {
		common.JSONError(w, http.StatusBadRequest, "内容过大（超过 1MB）")
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "保存失败: "+err.Error())
		return
	}

	common.JSONSuccess(w, map[string]string{"path": req.Path})
}

// resolveProjectPath 从请求中解析工作区并返回项目路径
func (c *WorkbenchCtrl) resolveProjectPath(w http.ResponseWriter, r *http.Request) (projectPath string, wsID uint, ok bool) {
	wsIDStr := r.URL.Query().Get("workspace_id")
	if wsIDStr == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 不能为空")
		return "", 0, false
	}

	id, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工作区 ID")
		return "", 0, false
	}

	workspace, err := c.WorkspaceService.GetByID(uint(id))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return "", 0, false
	}

	if workspace.ProjectID == 0 || workspace.Project.Path == "" {
		common.JSONSuccess(w, map[string]interface{}{"files": []fileNode{}})
		return "", 0, false
	}

	return workspace.Project.Path, uint(id), true
}

// validateProjectFilePath 校验项目文件路径安全性
func (c *WorkbenchCtrl) validateProjectFilePath(w http.ResponseWriter, projectPath, filePath string) (string, bool) {
	cleaned := filepath.Clean(filePath)
	if strings.Contains(cleaned, "..") || filepath.IsAbs(cleaned) {
		common.JSONError(w, http.StatusBadRequest, "非法路径")
		return "", false
	}

	fullPath := filepath.Join(projectPath, cleaned)
	absProject, _ := filepath.Abs(projectPath)
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absProject) {
		common.JSONError(w, http.StatusBadRequest, "路径超出项目范围")
		return "", false
	}

	return fullPath, true
}
