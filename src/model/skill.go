package model

// Skill 技能模型
type Skill struct {
	BaseModel
	Name        string `gorm:"size:100;uniqueIndex;not null" json:"name" validate:"required,min=2,max=100"`
	Label       string `gorm:"size:100;not null" json:"label" validate:"required,max=100"`
	Description string `gorm:"size:1000" json:"description" validate:"max=1000"`
	Keywords    string `gorm:"size:500" json:"keywords" validate:"max=500"`
	Content     string `gorm:"type:text" json:"content"`
	Level       int    `gorm:"default:1" json:"level"`  // 1=基础 2=进阶 3=专家
	Status      int    `gorm:"default:1" json:"status"` // 1=启用 0=禁用
	Sort        int    `gorm:"default:0" json:"sort"`
}

func (Skill) TableName() string { return "skills" }

// SkillCreateReq 创建技能请求
type SkillCreateReq struct {
	Name        string `validate:"required,min=2,max=100"`
	Label       string `validate:"required,max=100"`
	Description string `validate:"max=1000"`
	Keywords    string `validate:"max=500"`
	Content     string
	Level       int
}

// SkillUpdateReq 更新技能请求
type SkillUpdateReq struct {
	Label       string `validate:"omitempty,max=100"`
	Description string `validate:"max=1000"`
	Keywords    string `validate:"max=500"`
	Content     string
	Level       *int `validate:"omitempty,oneof=1 2 3"`
	Status      *int `validate:"omitempty,oneof=0 1"`
}
