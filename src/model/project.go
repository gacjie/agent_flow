package model

// Project 项目模型
type Project struct {
	BaseModel
	Name        string `gorm:"size:100;uniqueIndex;not null" json:"name" validate:"required,min=2,max=100"`
	Description string `gorm:"size:1000" json:"description" validate:"max=1000"`
	Path        string `gorm:"size:500" json:"path"`
	Status      int    `gorm:"default:1" json:"status"` // 1=活跃 2=归档
	Sort        int    `gorm:"default:0" json:"sort"`
}

func (Project) TableName() string { return "projects" }

// ProjectCreateReq 创建项目请求
type ProjectCreateReq struct {
	Name        string `validate:"required,min=2,max=100"`
	Description string `validate:"max=1000"`
	Path        string `validate:"required,max=500"`
}

// ProjectUpdateReq 更新项目请求
type ProjectUpdateReq struct {
	Description string `validate:"max=1000"`
	Path        string `validate:"omitempty,max=500"`
	Status      *int   `validate:"omitempty,oneof=1 2"`
}
