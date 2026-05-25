package model

// Role 角色模型
type Role struct {
	BaseModel
	Name  string `gorm:"size:50;uniqueIndex;not null" json:"name"`
	Label string `gorm:"size:50;not null" json:"label"`
	Sort  int    `gorm:"default:0" json:"sort"`
}

func (Role) TableName() string { return "roles" }

// Permission 权限节点模型
type Permission struct {
	BaseModel
	Code  string `gorm:"size:50;uniqueIndex;not null" json:"code"`
	Label string `gorm:"size:50;not null" json:"label"`
	Group string `gorm:"size:50;not null;index" json:"group"`
}

func (Permission) TableName() string { return "permissions" }

// RolePermission 角色-权限关联（多对多）
type RolePermission struct {
	RoleID         uint   `gorm:"primaryKey" json:"role_id"`
	PermissionCode string `gorm:"primaryKey;size:50" json:"permission_code"`
}

func (RolePermission) TableName() string { return "role_permissions" }
