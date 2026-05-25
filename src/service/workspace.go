package service

import (
	"fmt"
	"os"
	"path/filepath"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// WorkspaceService 工作区业务逻辑
type WorkspaceService struct {
	DB         *gorm.DB
	WorkingMgr *WorkingDBManager
}

// NewWorkspaceService 创建工作区服务实例
func NewWorkspaceService(db *gorm.DB, workingMgr *WorkingDBManager) *WorkspaceService {
	return &WorkspaceService{DB: db, WorkingMgr: workingMgr}
}

// ListByProject 获取指定项目的工作区列表（分页）
func (s *WorkspaceService) ListByProject(projectID uint, page, pageSize int) ([]model.Workspace, int64, error) {
	var items []model.Workspace
	var total int64

	query := s.DB.Model(&model.Workspace{}).Where("project_id = ?", projectID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Preload("Agent").Offset(offset).Limit(pageSize).Order("id DESC").Find(&items).Error
	return items, total, err
}

// GetByID 根据 ID 获取工作区
func (s *WorkspaceService) GetByID(id uint) (*model.Workspace, error) {
	var item model.Workspace
	err := s.DB.Preload("Agent").Preload("Project").First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Create 创建工作区（自动生成 UUID，初始化目录和 session.db）
func (s *WorkspaceService) Create(req *model.WorkspaceCreateReq) (*model.Workspace, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	// 生成工作区 UUID
	uuid, err := common.GenerateUUID()
	if err != nil {
		return nil, fmt.Errorf("生成 UUID 失败: %w", err)
	}

	workspace := &model.Workspace{
		UUID:        uuid,
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Label:       req.Label,
		Description: req.Description,
		AgentID:     req.AgentID,
		Status:      1,
	}

	if err := s.DB.Create(workspace).Error; err != nil {
		return nil, err
	}

	// 创建工作区目录
	workDir := s.GetWorkDir(uuid)
	os.MkdirAll(workDir, 0755)

	// 预创建 working.db（初始化表结构）
	if s.WorkingMgr != nil {
		if _, err := s.WorkingMgr.GetDB(uuid); err != nil {
			// 非致命错误，记录日志但不中断
			fmt.Printf("警告: 初始化 working.db 失败: %v\n", err)
		}
	}

	return workspace, nil
}

// GetWorkDir 获取工作区目录路径（按 UUID）
func (s *WorkspaceService) GetWorkDir(uuid string) string {
	return filepath.Join(config.Get().Agent.WorkingRoot, uuid)
}

// GetWorkDirByID 根据工作区 ID 获取工作目录路径
func (s *WorkspaceService) GetWorkDirByID(id uint) (string, error) {
	ws, err := s.GetByID(id)
	if err != nil {
		return "", err
	}
	return s.GetWorkDir(ws.UUID), nil
}

// ListActive 获取所有进行中的工作区（不分页）
func (s *WorkspaceService) ListActive() ([]model.Workspace, error) {
	var items []model.Workspace
	err := s.DB.Preload("Agent").Preload("Project").
		Where("status = 1").Order("id DESC").Find(&items).Error
	return items, err
}

// ListAll 获取所有工作区（分页，不限项目）
func (s *WorkspaceService) ListAll(page, pageSize int) ([]model.Workspace, int64, error) {
	var items []model.Workspace
	var total int64

	query := s.DB.Model(&model.Workspace{})
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Preload("Agent").Preload("Project").
		Offset(offset).Limit(pageSize).Order("id DESC").Find(&items).Error
	return items, total, err
}

// Update 更新工作区
func (s *WorkspaceService) Update(id uint, req *model.WorkspaceUpdateReq) (*model.Workspace, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	workspace, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Label != "" {
		updates["label"] = req.Label
	}
	updates["description"] = req.Description
	if req.AgentID != nil {
		updates["agent_id"] = *req.AgentID
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		if err := s.DB.Model(workspace).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetByID(id)
}

// Delete 删除工作区（软删除 + 清理磁盘目录和 DB 连接缓存）
func (s *WorkspaceService) Delete(id uint) error {
	ws, err := s.GetByID(id)
	if err != nil {
		return err
	}

	if err := s.DB.Delete(&model.Workspace{}, id).Error; err != nil {
		return err
	}

	// 关闭并移除工作区 DB 连接缓存
	if s.WorkingMgr != nil {
		s.WorkingMgr.CloseWorkspace(ws.UUID)
	}

	// 删除工作区目录（含 working.db 和所有工作文档）
	workDir := s.GetWorkDir(ws.UUID)
	os.RemoveAll(workDir)

	return nil
}

// Count 获取工作区总数
func (s *WorkspaceService) Count() int64 {
	var count int64
	s.DB.Model(&model.Workspace{}).Count(&count)
	return count
}

// CountByProject 获取指定项目的工作区数量
func (s *WorkspaceService) CountByProject(projectID uint) int64 {
	var count int64
	s.DB.Model(&model.Workspace{}).Where("project_id = ?", projectID).Count(&count)
	return count
}
