package service

import (
	"agent_flow/src/common"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// SkillService 技能业务逻辑
type SkillService struct {
	DB       *gorm.DB
	FileSync *FileSyncService
}

// NewSkillService 创建技能服务实例
func NewSkillService(db *gorm.DB, fileSync *FileSyncService) *SkillService {
	return &SkillService{DB: db, FileSync: fileSync}
}

// List 获取技能列表（分页）
func (s *SkillService) List(page, pageSize int) ([]model.Skill, int64, error) {
	var items []model.Skill
	var total int64

	query := s.DB.Model(&model.Skill{})

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&items).Error
	return items, total, err
}

// GetByID 根据 ID 获取技能
func (s *SkillService) GetByID(id uint) (*model.Skill, error) {
	var item model.Skill
	err := s.DB.First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Create 创建技能
func (s *SkillService) Create(req *model.SkillCreateReq) (*model.Skill, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	skill := &model.Skill{
		Name:        req.Name,
		Label:       req.Label,
		Description: req.Description,
		Keywords:    req.Keywords,
		Content:     req.Content,
		Level:       req.Level,
		Status:      1,
	}

	if skill.Level < 1 || skill.Level > 3 {
		skill.Level = 1
	}

	if err := s.DB.Create(skill).Error; err != nil {
		return nil, err
	}

	// 同步到文件系统
	if s.FileSync != nil {
		go s.FileSync.SyncSkill(skill)
	}

	return skill, nil
}

// Update 更新技能
func (s *SkillService) Update(id uint, req *model.SkillUpdateReq) (*model.Skill, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	skill, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Label != "" {
		updates["label"] = req.Label
	}
	updates["description"] = req.Description
	updates["keywords"] = req.Keywords
	updates["content"] = req.Content
	if req.Level != nil {
		updates["level"] = *req.Level
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		if err := s.DB.Model(skill).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	result, err := s.GetByID(id)
	if err == nil && s.FileSync != nil {
		go s.FileSync.SyncSkill(result)
	}
	return result, err
}

// Delete 删除技能（软删除）
func (s *SkillService) Delete(id uint) error {
	// 先获取名称用于删除文件
	skill, _ := s.GetByID(id)

	err := s.DB.Delete(&model.Skill{}, id).Error

	if err == nil && skill != nil && s.FileSync != nil {
		go s.FileSync.DeleteSkill(skill.Name)
	}
	return err
}

// Count 获取技能总数
func (s *SkillService) Count() int64 {
	var count int64
	s.DB.Model(&model.Skill{}).Count(&count)
	return count
}

// ListAll 返回所有启用的技能，按 sort 排序
func (s *SkillService) ListAll() ([]model.Skill, error) {
	var items []model.Skill
	err := s.DB.Where("status = ?", 1).Order("sort ASC, id ASC").Find(&items).Error
	return items, err
}
