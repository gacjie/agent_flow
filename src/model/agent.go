package model

// Agent AI 智能体模型（内容字段存储于 context/agents/{name}/ 文件系统）
type Agent struct {
	BaseModel
	Name          string   `gorm:"size:100;uniqueIndex;not null" json:"name" validate:"required,min=2,max=100"`
	Title         string   `gorm:"size:100;not null" json:"title" validate:"required,max=100"`
	Description   string   `gorm:"size:1000" json:"description" validate:"max=1000"`
	Keywords      string   `gorm:"size:500" json:"keywords" validate:"max=500"`
	ModelID       string   `gorm:"size:100;default:auto" json:"model_id"`              // 模型标识：auto/default/具体model_id
	Model         LLMModel `gorm:"foreignKey:ModelID;references:ModelID" json:"model"` // 外键指向 llm_models.model_id
	ModelMode     string   `gorm:"size:20;default:auto" json:"model_mode"`             // auto=自动切换, default=默认模型
	AutoLoadFiles string   `gorm:"size:1000" json:"auto_load_files"`
	AutoLoadTools string   `gorm:"size:1000" json:"auto_load_tools"`
	DocRoles      string   `gorm:"size:200" json:"doc_roles"`
	Icon          string   `gorm:"size:50" json:"icon"`
	Status        int      `gorm:"default:1" json:"status"`
	Sort          int      `gorm:"default:0" json:"sort"`
}

func (Agent) TableName() string { return "agents" }

func (a *Agent) DisplayName() string {
	if a.Title != "" && a.Name != "" {
		return a.Title + "-" + a.Name
	}
	if a.Title != "" {
		return a.Title
	}
	return a.Name
}

// AgentSkill 智能体-技能关联（多对多）
type AgentSkill struct {
	AgentID uint `gorm:"primaryKey"`
	SkillID uint `gorm:"primaryKey"`
	Sort    int  `gorm:"default:0"`
}

func (AgentSkill) TableName() string { return "agent_skills" }

// AgentCreateReq 创建 AI 智能体请求
// 内容字段（RolePrompt 等）由 Service 层写入 context/agents/{name}/*.md 文件
type AgentCreateReq struct {
	Name            string `validate:"required,min=2,max=100"`
	Title           string `validate:"required,max=100"`
	Description     string `validate:"max=1000"`
	Keywords        string `validate:"max=500"`
	ModelID         string
	ModelMode       string
	RolePrompt      string
	MemoryContent   string
	RuleContent     string
	SkillContent    string
	WorkflowContent string
	AutoLoadFiles   string
	AutoLoadTools   string
	DocRoles        string
	Icon            string
	SkillIDs        []uint // 关联技能 ID 列表
}

// AgentUpdateReq 更新 AI 智能体请求
type AgentUpdateReq struct {
	Title           string `validate:"omitempty,max=100"`
	Description     string `validate:"max=1000"`
	Keywords        string `validate:"max=500"`
	ModelID         *string
	ModelMode       *string
	RolePrompt      *string
	MemoryContent   *string
	RuleContent     *string
	SkillContent    *string
	WorkflowContent *string
	AutoLoadFiles   *string
	AutoLoadTools   *string
	DocRoles        *string
	Icon            *string
	Status          *int `validate:"omitempty,oneof=0 1"`
	SkillIDs        []uint
}
