package service

import (
	"net/http"
	"path/filepath"
	"strings"

	"agent_flow/src/common"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// ProjectService 项目业务逻辑
type ProjectService struct {
	DB *gorm.DB
}

// NewProjectService 创建项目服务实例
func NewProjectService(db *gorm.DB) *ProjectService {
	return &ProjectService{DB: db}
}

// List 获取项目列表（分页）
func (s *ProjectService) List(page, pageSize int) ([]model.Project, int64, error) {
	var items []model.Project
	var total int64

	s.DB.Model(&model.Project{}).Count(&total)

	offset := (page - 1) * pageSize
	err := s.DB.Offset(offset).Limit(pageSize).Order("sort ASC, id DESC").Find(&items).Error
	return items, total, err
}

// GetByID 根据 ID 获取项目
func (s *ProjectService) GetByID(id uint) (*model.Project, error) {
	var item model.Project
	err := s.DB.First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Create 创建项目
func (s *ProjectService) Create(req *model.ProjectCreateReq) (*model.Project, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	projectPath := filepath.Clean(req.Path)

	project := &model.Project{
		Name:        req.Name,
		Description: req.Description,
		Path:        projectPath,
		Status:      1,
	}

	if err := s.DB.Create(project).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return nil, common.NewError(http.StatusBadRequest, "项目名称已存在")
		}
		return nil, err
	}

	return project, nil
}

// Update 更新项目
func (s *ProjectService) Update(id uint, req *model.ProjectUpdateReq) (*model.Project, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	project, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Path != "" {
		updates["path"] = filepath.Clean(req.Path)
	}
	updates["description"] = req.Description
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		if err := s.DB.Model(project).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetByID(id)
}

// Delete 删除项目（软删除）
func (s *ProjectService) Delete(id uint) error {
	return s.DB.Delete(&model.Project{}, id).Error
}

// Count 获取项目总数
func (s *ProjectService) Count() int64 {
	var count int64
	s.DB.Model(&model.Project{}).Count(&count)
	return count
}

// ListAll 返回所有活跃项目（下拉选择用）
func (s *ProjectService) ListAll() ([]model.Project, error) {
	var items []model.Project
	err := s.DB.Where("status = ?", 1).Order("sort ASC, id ASC").Find(&items).Error
	return items, err
}
