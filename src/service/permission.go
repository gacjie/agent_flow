package service

import (
	"log/slog"
	"sync"

	"agent_flow/src/model"

	"gorm.io/gorm"
)

// PermissionService 权限业务逻辑（含内存缓存）
type PermissionService struct {
	DB    *gorm.DB
	mu    sync.RWMutex
	cache map[uint]map[string]bool // roleID → {permCode: true}
}

// NewPermissionService 创建权限服务并加载缓存
func NewPermissionService(db *gorm.DB) *PermissionService {
	s := &PermissionService{DB: db, cache: make(map[uint]map[string]bool)}
	s.Reload()
	return s
}

// Reload 重新加载角色-权限映射到内存
func (s *PermissionService) Reload() {
	var rps []model.RolePermission
	s.DB.Find(&rps)

	newCache := make(map[uint]map[string]bool)
	for _, rp := range rps {
		if newCache[rp.RoleID] == nil {
			newCache[rp.RoleID] = make(map[string]bool)
		}
		newCache[rp.RoleID][rp.PermissionCode] = true
	}

	s.mu.Lock()
	s.cache = newCache
	s.mu.Unlock()

	slog.Debug("权限缓存已刷新", "roles", len(newCache))
}

// HasPermission 检查角色是否拥有指定权限
func (s *PermissionService) HasPermission(roleID uint, code string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	perms, ok := s.cache[roleID]
	if !ok {
		return false
	}
	return perms[code]
}

// GetPermissionsByRole 获取角色的所有权限 Code 列表
func (s *PermissionService) GetPermissionsByRole(roleID uint) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	perms, ok := s.cache[roleID]
	if !ok {
		return nil
	}
	codes := make([]string, 0, len(perms))
	for code := range perms {
		codes = append(codes, code)
	}
	return codes
}

// GetPermissionMap 获取角色权限的 map（模板渲染用）
func (s *PermissionService) GetPermissionMap(roleID uint) map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	perms, ok := s.cache[roleID]
	if !ok {
		return make(map[string]bool)
	}
	// 返回副本
	result := make(map[string]bool, len(perms))
	for k, v := range perms {
		result[k] = v
	}
	return result
}

// SetRolePermissions 设置角色权限（全量替换，含白名单校验）
func (s *PermissionService) SetRolePermissions(roleID uint, codes []string) error {
	// 查询所有有效权限码作为白名单
	var validPerms []model.Permission
	s.DB.Select("code").Find(&validPerms)
	validCodes := make(map[string]bool, len(validPerms))
	for _, p := range validPerms {
		validCodes[p.Code] = true
	}

	// 过滤非法权限码
	var filtered []string
	for _, code := range codes {
		if validCodes[code] {
			filtered = append(filtered, code)
		}
	}

	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&model.RolePermission{}).Error; err != nil {
			return err
		}
		for _, code := range filtered {
			rp := model.RolePermission{RoleID: roleID, PermissionCode: code}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ListRoles 获取所有角色
func (s *PermissionService) ListRoles() ([]model.Role, error) {
	var roles []model.Role
	err := s.DB.Order("sort ASC, id ASC").Find(&roles).Error
	return roles, err
}

// GetRole 获取单个角色
func (s *PermissionService) GetRole(id uint) (*model.Role, error) {
	var role model.Role
	err := s.DB.First(&role, id).Error
	return &role, err
}

// ListPermissions 获取所有权限节点
func (s *PermissionService) ListPermissions() ([]model.Permission, error) {
	var perms []model.Permission
	err := s.DB.Order("`group` ASC, id ASC").Find(&perms).Error
	return perms, err
}

// GetRolePermissionCodes 获取角色已关联的权限 Code（从数据库）
func (s *PermissionService) GetRolePermissionCodes(roleID uint) ([]string, error) {
	var rps []model.RolePermission
	err := s.DB.Where("role_id = ?", roleID).Find(&rps).Error
	if err != nil {
		return nil, err
	}
	codes := make([]string, len(rps))
	for i, rp := range rps {
		codes[i] = rp.PermissionCode
	}
	return codes, nil
}
