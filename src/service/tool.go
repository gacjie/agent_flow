package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"agent_flow/src/model"
	"agent_flow/src/tool"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ToolInfo 工具信息（统一视图：系统工具+MCP工具）
type ToolInfo struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	IsBuiltin   bool   `json:"is_builtin"` // 系统内置工具（Go 实现）
	IsMCP       bool   `json:"is_mcp"`     // MCP 工具（外部命令）
	Status      int    `json:"status"`     // 1=启用 0=禁用
}

// ToolService 工具管理服务
type ToolService struct {
	DB         *gorm.DB
	Registry   *tool.Registry
	MCPManager *tool.MCPManager
	mu         sync.RWMutex
	cache      []ToolInfo // 所有可用工具的缓存
}

// NewToolService 创建工具服务
func NewToolService(db *gorm.DB, registry *tool.Registry) *ToolService {
	return &ToolService{
		DB:       db,
		Registry: registry,
	}
}

// SyncBuiltins 将 Registry 中所有工具 Upsert 到 system_tools 表
// 必须在 router.New() 之后调用（确保管理工具已注册到 Registry）
func (s *ToolService) SyncBuiltins(registry *tool.Registry) error {
	tools := registry.List()
	for i, t := range tools {
		record := model.SystemTool{
			Name:        t.Name(),
			Label:       t.Name(),
			Description: t.Description(),
			Category:    "内置",
			Sort:        i,
		}
		if err := s.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"label", "description", "category", "sort"}),
		}).Create(&record).Error; err != nil {
			slog.Error("系统工具同步失败", "name", t.Name(), "error", err)
			continue
		}
	}
	s.refreshCache()
	slog.Info("系统工具同步完成", "count", len(tools))
	return nil
}

// SyncMCPEnabled 启动所有已启用的 MCP 服务器（应用启动时调用）
func (s *ToolService) SyncMCPEnabled(registry *tool.Registry) error {
	if s.MCPManager == nil {
		return nil
	}
	var mcpTools []model.MCPTool
	if err := s.DB.Where("status = ?", 1).Order("sort ASC, id ASC").Find(&mcpTools).Error; err != nil {
		return err
	}
	started := 0
	for _, t := range mcpTools {
		t := t
		if err := s.MCPManager.StartServer(&t); err != nil {
			slog.Warn("MCP 服务器启动失败", "name", t.Name, "error", err)
			continue
		}
		started++
	}
	s.refreshCache()
	slog.Info("MCP 服务器同步完成", "total", len(mcpTools), "started", started)
	return nil
}

// ListAvailable 列出所有可用工具信息（系统工具+MCP工具）
func (s *ToolService) ListAvailable() []ToolInfo {
	s.mu.RLock()
	if len(s.cache) > 0 {
		defer s.mu.RUnlock()
		return s.cache
	}
	s.mu.RUnlock()
	s.refreshCache()
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache
}

// ListSystemTools 列出所有系统工具（从数据库）
func (s *ToolService) ListSystemTools() ([]model.SystemTool, error) {
	var tools []model.SystemTool
	if err := s.DB.Order("sort ASC, id ASC").Find(&tools).Error; err != nil {
		return nil, err
	}
	return tools, nil
}

// ToggleSystemToolStatus 切换系统工具启用/禁用状态
func (s *ToolService) ToggleSystemToolStatus(id uint) error {
	var t model.SystemTool
	if err := s.DB.First(&t, id).Error; err != nil {
		return err
	}
	newStatus := 1
	if t.Status == 1 {
		newStatus = 0
	}
	if err := s.DB.Model(&t).Update("status", newStatus).Error; err != nil {
		return err
	}
	s.refreshCache()
	return nil
}

// ListMCPTools 列出所有 MCP 工具（从数据库）
func (s *ToolService) ListMCPTools() ([]model.MCPTool, error) {
	var tools []model.MCPTool
	if err := s.DB.Order("sort ASC, id ASC").Find(&tools).Error; err != nil {
		return nil, err
	}
	return tools, nil
}

// GetMCPTool 按 ID 获取 MCP 工具
func (s *ToolService) GetMCPTool(id uint) (*model.MCPTool, error) {
	var t model.MCPTool
	if err := s.DB.First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateMCPTool 创建 MCP 工具
func (s *ToolService) CreateMCPTool(req *model.MCPToolCreateReq) error {
	if err := validateMCPToolJSON(req.Args, req.Env, req.Parameters); err != nil {
		return err
	}
	t := model.MCPTool{
		Name:        req.Name,
		Label:       req.Label,
		Description: req.Description,
		Command:     req.Command,
		Args:        req.Args,
		Env:         req.Env,
		Parameters:  req.Parameters,
		Category:    req.Category,
		Version:     req.Version,
		Timeout:     req.Timeout,
		Status:      1,
	}
	if t.Timeout <= 0 {
		t.Timeout = 30
	}
	if err := s.DB.Create(&t).Error; err != nil {
		return err
	}
	if s.MCPManager != nil && t.Status == 1 {
		if err := s.MCPManager.StartServer(&t); err != nil {
			slog.Warn("MCP 服务器自动启动失败", "name", t.Name, "error", err)
		}
	}
	s.refreshCache()
	return nil
}

// UpdateMCPTool 更新 MCP 工具
func (s *ToolService) UpdateMCPTool(id uint, req *model.MCPToolUpdateReq) error {
	var t model.MCPTool
	if err := s.DB.First(&t, id).Error; err != nil {
		return err
	}
	t.Label = req.Label
	t.Description = req.Description
	t.Command = req.Command
	t.Args = req.Args
	t.Env = req.Env
	t.Parameters = req.Parameters
	t.Category = req.Category
	t.Version = req.Version
	t.Timeout = req.Timeout
	if t.Timeout <= 0 {
		t.Timeout = 30
	}
	if err := validateMCPToolJSON(req.Args, req.Env, req.Parameters); err != nil {
		return err
	}
	if err := s.DB.Save(&t).Error; err != nil {
		return err
	}
	if s.MCPManager != nil && t.Status == 1 {
		if err := s.MCPManager.RestartServer(&t); err != nil {
			slog.Warn("MCP 服务器重启失败", "name", t.Name, "error", err)
		}
	}
	s.refreshCache()
	return nil
}

// DeleteMCPTool 删除 MCP 工具
func (s *ToolService) DeleteMCPTool(id uint) error {
	var t model.MCPTool
	if err := s.DB.First(&t, id).Error; err != nil {
		return err
	}
	if s.MCPManager != nil {
		s.MCPManager.StopServer(t.Name)
	}
	if err := s.DB.Delete(&t).Error; err != nil {
		return err
	}
	s.refreshCache()
	return nil
}

// ToggleMCPStatus 切换 MCP 工具启用/禁用状态
func (s *ToolService) ToggleMCPStatus(id uint) error {
	var t model.MCPTool
	if err := s.DB.First(&t, id).Error; err != nil {
		return err
	}
	newStatus := 1
	if t.Status == 1 {
		newStatus = 0
	}
	if err := s.DB.Model(&t).Update("status", newStatus).Error; err != nil {
		return err
	}
	if s.MCPManager != nil {
		if newStatus == 1 {
			t.Status = 1
			if err := s.MCPManager.StartServer(&t); err != nil {
				slog.Warn("MCP 服务器启动失败", "name", t.Name, "error", err)
			}
		} else {
			s.MCPManager.StopServer(t.Name)
		}
	}
	s.refreshCache()
	return nil
}

// GetToolsByNames 按名称列表获取工具（用于 Agent 的 AutoLoadTools 过滤）
func (s *ToolService) GetToolsByNames(names []string) []tool.Tool {
	if len(names) == 0 {
		return nil
	}
	for _, n := range names {
		if n == "all" {
			return s.Registry.List()
		}
	}
	var result []tool.Tool
	for _, name := range names {
		if t, ok := s.Registry.Get(name); ok {
			result = append(result, t)
		}
	}
	return result
}

// StartMCPServer 启动指定 MCP 服务器
func (s *ToolService) StartMCPServer(id uint) error {
	t, err := s.GetMCPTool(id)
	if err != nil {
		return err
	}
	if s.MCPManager == nil {
		return fmt.Errorf("MCP 管理器未初始化")
	}
	if err := s.MCPManager.StartServer(t); err != nil {
		return err
	}
	s.refreshCache()
	return nil
}

// StopMCPServer 停止指定 MCP 服务器
func (s *ToolService) StopMCPServer(id uint) error {
	t, err := s.GetMCPTool(id)
	if err != nil {
		return err
	}
	if s.MCPManager == nil {
		return fmt.Errorf("MCP 管理器未初始化")
	}
	if err := s.MCPManager.StopServer(t.Name); err != nil {
		return err
	}
	s.refreshCache()
	return nil
}

// GetMCPStatuses 获取所有 MCP 服务器运行状态
func (s *ToolService) GetMCPStatuses() map[string]tool.MCPServerStatus {
	if s.MCPManager == nil {
		return nil
	}
	return s.MCPManager.GetAllStatuses()
}

// refreshCache 刷新工具缓存（从 system_tools + mcp_tools 表读取）
func (s *ToolService) refreshCache() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var infos []ToolInfo

	// 系统工具（从 system_tools 表）
	var systemTools []model.SystemTool
	s.DB.Order("sort ASC, id ASC").Find(&systemTools)
	for _, t := range systemTools {
		infos = append(infos, ToolInfo{
			ID:          t.ID,
			Name:        t.Name,
			Label:       t.Label,
			Description: t.Description,
			Category:    t.Category,
			IsBuiltin:   true,
			IsMCP:       false,
			Status:      t.Status,
		})
	}

	// MCP 工具（从 mcp_tools 表）
	var mcpTools []model.MCPTool
	s.DB.Order("sort ASC, id ASC").Find(&mcpTools)
	for _, t := range mcpTools {
		infos = append(infos, ToolInfo{
			ID:          t.ID,
			Name:        t.Name,
			Label:       t.Label,
			Description: t.Description,
			Category:    t.Category,
			Version:     t.Version,
			IsBuiltin:   false,
			IsMCP:       true,
			Status:      t.Status,
		})
	}

	s.cache = infos
}

// ParseAutoLoadTools 解析 AutoLoadTools JSON 字符串为名称列表
func ParseAutoLoadTools(jsonStr string) []string {
	if jsonStr == "" {
		return nil
	}
	var names []string
	if err := json.Unmarshal([]byte(jsonStr), &names); err != nil {
		return nil
	}
	return names
}

// MCPImportResult 批量导入 MCP 工具的结果
type MCPImportResult struct {
	Created      int      `json:"created"`
	Skipped      int      `json:"skipped"`
	SkippedNames []string `json:"skipped_names,omitempty"`
	Errors       []string `json:"errors,omitempty"`
}

// GetMCPToolByName 按名称查询 MCP 工具
func (s *ToolService) GetMCPToolByName(name string) (*model.MCPTool, error) {
	var t model.MCPTool
	if err := s.DB.Where("name = ?", name).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// GetMCPToolsByIDs 按 ID 批量查询 MCP 工具
func (s *ToolService) GetMCPToolsByIDs(ids []uint) ([]model.MCPTool, error) {
	var items []model.MCPTool
	if len(ids) == 0 {
		return items, nil
	}
	err := s.DB.Where("id IN ?", ids).Order("id ASC").Find(&items).Error
	return items, err
}

// ImportMCPTools 批量导入 MCP 工具，已存在的按名称跳过
func (s *ToolService) ImportMCPTools(items []model.MCPToolCreateReq) (*MCPImportResult, error) {
	result := &MCPImportResult{}
	for _, req := range items {
		if _, err := s.GetMCPToolByName(req.Name); err == nil {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, req.Name)
			continue
		}
		if err := s.CreateMCPTool(&req); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", req.Name, err.Error()))
			continue
		}
		result.Created++
	}
	return result, nil
}

func validateMCPToolJSON(args, env, parameters string) error {
	if args != "" {
		var v []string
		if err := json.Unmarshal([]byte(args), &v); err != nil {
			return fmt.Errorf("Args 不是有效的 JSON 数组: %w", err)
		}
	}
	if env != "" {
		var v map[string]string
		if err := json.Unmarshal([]byte(env), &v); err != nil {
			return fmt.Errorf("Env 不是有效的 JSON 对象: %w", err)
		}
	}
	if parameters != "" {
		var v json.RawMessage
		if err := json.Unmarshal([]byte(parameters), &v); err != nil {
			return fmt.Errorf("Parameters 不是有效的 JSON: %w", err)
		}
	}
	return nil
}
