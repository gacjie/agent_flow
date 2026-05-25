package service

import (
	"agent_flow/src/common"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// AdminService 管理员业务逻辑
type AdminService struct {
	DB *gorm.DB
}

// NewAdminService 创建管理员服务实例
func NewAdminService(db *gorm.DB) *AdminService {
	return &AdminService{DB: db}
}

// List 获取管理员列表（分页，预加载角色）
func (s *AdminService) List(page, pageSize int) ([]model.Admin, int64, error) {
	var admins []model.Admin
	var total int64

	s.DB.Model(&model.Admin{}).Count(&total)

	offset := (page - 1) * pageSize
	err := s.DB.Preload("Role").Offset(offset).Limit(pageSize).Order("id DESC").Find(&admins).Error
	return admins, total, err
}

// GetByID 根据 ID 获取管理员（预加载角色）
func (s *AdminService) GetByID(id uint) (*model.Admin, error) {
	var admin model.Admin
	err := s.DB.Preload("Role").First(&admin, id).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// Create 创建管理员
func (s *AdminService) Create(req *model.AdminCreateReq) (*model.Admin, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	hashedPwd, err := common.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	admin := &model.Admin{
		Username: req.Username,
		Email:    req.Email,
		Password: hashedPwd,
		Nickname: req.Nickname,
		RoleID:   req.RoleID,
		Status:   1,
	}

	if err := s.DB.Create(admin).Error; err != nil {
		return nil, err
	}
	return admin, nil
}

// Update 更新管理员
func (s *AdminService) Update(id uint, req *model.AdminUpdateReq) (*model.Admin, error) {
	if err := common.ValidateStruct(req); err != nil {
		return nil, err
	}

	admin, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if req.RoleID != nil {
		updates["role_id"] = *req.RoleID
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		if err := s.DB.Model(admin).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetByID(id)
}

// UpdateProfile 更新个人资料
func (s *AdminService) UpdateProfile(id uint, req *model.ProfileUpdateReq) error {
	admin, err := s.GetByID(id)
	if err != nil {
		return err
	}

	updates := map[string]interface{}{}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}

	if req.NewPassword != "" {
		if !common.CheckPassword(req.OldPassword, admin.Password) {
			return common.NewError(400, "原密码不正确")
		}
		hashedPwd, err := common.HashPassword(req.NewPassword)
		if err != nil {
			return err
		}
		updates["password"] = hashedPwd
	}

	if len(updates) > 0 {
		return s.DB.Model(admin).Updates(updates).Error
	}
	return nil
}

// ClearMustChangePassword 清除强制改密标记
func (s *AdminService) ClearMustChangePassword(id uint) {
	s.DB.Model(&model.Admin{}).Where("id = ?", id).Update("must_change_password", false)
}

// Delete 删除管理员（软删除）
func (s *AdminService) Delete(id uint) error {
	return s.DB.Delete(&model.Admin{}, id).Error
}

// Count 获取管理员总数
func (s *AdminService) Count() int64 {
	var count int64
	s.DB.Model(&model.Admin{}).Count(&count)
	return count
}
