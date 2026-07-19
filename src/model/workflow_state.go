package model

// WorkflowState 工作流状态（存储在工作区 working.db）
type WorkflowState struct {
	BaseModel
	ConversationID uint   `gorm:"uniqueIndex;not null" json:"conversation_id"`
	AgentName      string `gorm:"size:100;not null" json:"agent_name"`
	CurrentStepNum int    `gorm:"default:0" json:"current_step_num"`
	TotalSteps     int    `gorm:"not null" json:"total_steps"`
	Status         int    `gorm:"default:0" json:"status"`      // 0=空闲 1=进行中 2=已完成
	RoundNum       int    `gorm:"default:1" json:"round_num"`   // 当前轮次（1-based，每次工作流完成后用户再发消息自动+1）
	Corrections       int    `gorm:"default:0" json:"corrections"` // 当前步骤纠正次数
	StepHistory       string `gorm:"type:text" json:"step_history"`
	LastContext       string `gorm:"type:text" json:"last_context"`        // 上一步传递的 context_for_next
	TruncateRequested bool   `gorm:"-" json:"-"`                           // 非持久化标记：工具请求截断
}

func (WorkflowState) TableName() string { return "workflow_states" }

// StepHistoryEntry 步骤执行历史条目
type StepHistoryEntry struct {
	Round          int    `json:"round,omitempty"`
	Step           int    `json:"step"`
	Label          string `json:"label"`
	Summary        string `json:"summary,omitempty"`
	ContextForNext string `json:"context_for_next,omitempty"`
}
