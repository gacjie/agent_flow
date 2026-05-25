package model

// Workspace 工作区模型
type Workspace struct {
	BaseModel
	UUID        string  `gorm:"size:36;uniqueIndex;not null" json:"uuid"`
	ProjectID   uint    `gorm:"default:0;index" json:"project_id"`
	Project     Project `gorm:"foreignKey:ProjectID" json:"project"`
	Name        string  `gorm:"size:100;not null" json:"name" validate:"required,min=2,max=100"`
	Label       string  `gorm:"size:100;not null" json:"label" validate:"required,max=100"`
	Description string  `gorm:"size:1000" json:"description" validate:"max=1000"`
	AgentID     uint    `gorm:"default:0" json:"agent_id"`
	Agent       Agent   `gorm:"foreignKey:AgentID" json:"agent"`
	Status      int     `gorm:"default:1" json:"status"` // 1=进行中 2=完成 3=暂停
}

func (Workspace) TableName() string { return "workspaces" }

// WorkspaceCreateReq 创建工作区请求
type WorkspaceCreateReq struct {
	ProjectID   uint   `validate:"omitempty"`
	Name        string `validate:"required,min=2,max=100"`
	Label       string `validate:"omitempty,max=100"` // 可选，不传时由控制器用 Name 填充
	Description string `validate:"max=1000"`
	AgentID     uint
}

// WorkspaceUpdateReq 更新工作区请求
type WorkspaceUpdateReq struct {
	Label       string `validate:"omitempty,max=100"`
	Description string `validate:"max=1000"`
	AgentID     *uint
	Status      *int `validate:"omitempty,oneof=1 2 3"`
}
