package controller

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"agent_flow/src/tokenutil"
	"agent_flow/src/common"
)

type docFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	SizeText string `json:"size_text"`
	ModTime  string `json:"mod_time"`
	Scope    string `json:"scope"`
	Tokens   int    `json:"tokens"`
}

func formatSizeText(size int64) string {
	if size >= 1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	}
	return fmt.Sprintf("%dB", size)
}

func buildDocFile(name, path, scope string, info os.FileInfo, dir string) docFile {
	var tokens int
	if info.Size() > 0 && info.Size() <= 1<<20 {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			tokens = tokenutil.EstimateText(string(data))
		}
	}
	return docFile{
		Name:     name,
		Path:     path,
		Size:     info.Size(),
		SizeText: formatSizeText(info.Size()),
		ModTime:  info.ModTime().Format("2006-01-02 15:04"),
		Scope:    scope,
		Tokens:   tokens,
	}
}

// ListDocs 获取工作区文档列表（specs/ + tasks/）
func (c *WorkbenchCtrl) ListDocs(w http.ResponseWriter, r *http.Request) {
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

	var docs []docFile
	var totalTokens int
	for _, scope := range []string{"specs", "tasks"} {
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
			df := buildDocFile(e.Name(), scope+"/"+e.Name(), scope, info, dir)
			totalTokens += df.Tokens
			docs = append(docs, df)
		}
	}

	common.JSONSuccess(w, map[string]interface{}{
		"docs":         docs,
		"total_tokens": totalTokens,
	})
}

// ReadDoc 读取工作区文档内容
func (c *WorkbenchCtrl) ReadDoc(w http.ResponseWriter, r *http.Request) {
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

	docPath := r.URL.Query().Get("path")
	if docPath == "" {
		common.JSONError(w, http.StatusBadRequest, "path 不能为空")
		return
	}

	cleaned := filepath.Clean(docPath)
	if !strings.HasPrefix(cleaned, "specs"+string(filepath.Separator)) && !strings.HasPrefix(cleaned, "tasks"+string(filepath.Separator)) &&
		!strings.HasPrefix(cleaned, "specs/") && !strings.HasPrefix(cleaned, "tasks/") {
		common.JSONError(w, http.StatusBadRequest, "只允许访问 specs/ 或 tasks/ 目录下的文件")
		return
	}

	workDir := c.WorkspaceService.GetWorkDir(ws.UUID)
	fullPath := filepath.Join(workDir, cleaned)

	if !strings.HasPrefix(fullPath, workDir) {
		common.JSONError(w, http.StatusBadRequest, "非法路径")
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "文件不存在或读取失败")
		return
	}

	common.JSONSuccess(w, map[string]string{
		"path":    docPath,
		"content": string(data),
	})
}

// ListProjectDocs 获取项目文档列表（AGENTS.md、README.md + docs/*.md）
func (c *WorkbenchCtrl) ListProjectDocs(w http.ResponseWriter, r *http.Request) {
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

	if ws.ProjectID == 0 || ws.Project.Path == "" {
		common.JSONSuccess(w, map[string]interface{}{
			"docs":         []docFile{},
			"total_tokens": 0,
		})
		return
	}

	projectPath := ws.Project.Path
	var docs []docFile
	var totalTokens int

	// 扫描根目录固定文件
	for _, name := range []string{"AGENTS.md", "README.md"} {
		fullPath := filepath.Join(projectPath, name)
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			continue
		}
		df := buildDocFile(name, name, "root", info, projectPath)
		totalTokens += df.Tokens
		docs = append(docs, df)
	}

	// 扫描 docs/ 一级目录
	docsDir := filepath.Join(projectPath, "docs")
	entries, err := os.ReadDir(docsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			df := buildDocFile(e.Name(), "docs/"+e.Name(), "docs", info, docsDir)
			totalTokens += df.Tokens
			docs = append(docs, df)
		}
	}

	common.JSONSuccess(w, map[string]interface{}{
		"docs":         docs,
		"total_tokens": totalTokens,
	})
}

// ReadProjectDoc 读取项目文档内容
func (c *WorkbenchCtrl) ReadProjectDoc(w http.ResponseWriter, r *http.Request) {
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

	if ws.ProjectID == 0 || ws.Project.Path == "" {
		common.JSONError(w, http.StatusBadRequest, "工作区未关联项目")
		return
	}

	docPath := r.URL.Query().Get("path")
	if docPath == "" {
		common.JSONError(w, http.StatusBadRequest, "path 不能为空")
		return
	}

	// 白名单校验
	cleaned := filepath.Clean(docPath)
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
		common.JSONError(w, http.StatusBadRequest, "只允许访问 AGENTS.md、README.md 或 docs/ 目录下的 .md 文件")
		return
	}

	projectPath := ws.Project.Path
	fullPath := filepath.Join(projectPath, cleaned)

	// 防止路径遍历
	if !strings.HasPrefix(fullPath, projectPath) {
		common.JSONError(w, http.StatusBadRequest, "非法路径")
		return
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "文件不存在或读取失败")
		return
	}

	common.JSONSuccess(w, map[string]string{
		"path":    docPath,
		"content": string(data),
	})
}
