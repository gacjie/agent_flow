package model

// Admin 管理员模型
type Admin struct {
	BaseModel
	Username string `gorm:"size:50;uniqueIndex;not null" json:"username" validate:"required,min=2,max=50"`
	Email    string `gorm:"size:100;uniqueIndex;not null" json:"email" validate:"required,email"`
	Password string `gorm:"size:255;not null" json:"-" validate:"required,min=6"`
	Nickname string `gorm:"size:50" json:"nickname"`
	RoleID   uint   `gorm:"not null;index" json:"role_id"`
	Role     Role   `gorm:"foreignKey:RoleID" json:"role"`
	Status             int  `gorm:"default:1" json:"status"` // 1=启用 0=禁用
	MustChangePassword bool `gorm:"default:false" json:"-"`  // 首次登录须改密码
}

func (Admin) TableName() string { return "admins" }

// AdminCreateReq 创建管理员请求
type AdminCreateReq struct {
	Username string `json:"username" validate:"required,min=2,max=50"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Nickname string `json:"nickname" validate:"max=50"`
	RoleID   uint   `json:"role_id" validate:"required"`
}

// AdminUpdateReq 更新管理员请求
type AdminUpdateReq struct {
	Email    string `json:"email" validate:"omitempty,email"`
	Nickname string `json:"nickname" validate:"max=50"`
	RoleID   *uint  `json:"role_id" validate:"omitempty"`
	Status   *int   `json:"status" validate:"omitempty,oneof=0 1"`
}

// ProfileUpdateReq 个人资料更新请求
type ProfileUpdateReq struct {
	Email       string `json:"email" validate:"omitempty,email"`
	Nickname    string `json:"nickname" validate:"max=50"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password" validate:"omitempty,min=6"`
}
