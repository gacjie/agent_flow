package model

import "time"

// Conversation 对话/会话模型（存储在工作区 session.db）
type Conversation struct {
	BaseModel
	UUID        string     `gorm:"size:36;index" json:"uuid"`
	WorkspaceID uint       `gorm:"not null;index" json:"workspace_id"`
	AgentID     uint       `gorm:"default:0;index" json:"agent_id"` // 允许为0（团队模式主会话）
	AgentName   string     `gorm:"size:100" json:"agent_name"`      // 冗余存储，避免跨DB查询
	ParentID    uint       `gorm:"default:0;index" json:"parent_id"` // 子会话指向父会话
	ConvType    string     `gorm:"size:20;default:main" json:"conv_type"` // main=主会话 sub=子会话
	Title       string     `gorm:"size:200" json:"title"`
	Summary     string     `gorm:"type:text" json:"summary"`          // 阶段摘要
	Status      int        `gorm:"default:1;index" json:"status"`     // 1=进行中 2=已完成 3=已取消
	TotalTokens int        `gorm:"default:0" json:"total_tokens"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

func (Conversation) TableName() string { return "conversations" }

// ChatMessage 对话消息模型（存储在工作区 session.db）
type ChatMessage struct {
	BaseModel
	ConversationID   uint   `gorm:"not null;index" json:"conversation_id"`
	Role             string `gorm:"size:20;not null" json:"role"`          // system/user/assistant/tool
	Content          string `gorm:"type:text" json:"content"`              // 文本内容
	ReasoningContent string `gorm:"type:text" json:"reasoning_content"`    // 推理/思考内容
	ToolCalls        string `gorm:"type:text" json:"tool_calls"`           // JSON: []ToolCall
	ToolCallID       string `gorm:"size:100" json:"tool_call_id"`          // tool 消息的调用 ID
	Attachments      string `gorm:"type:text" json:"attachments"`          // JSON: []Attachment（图片/文件附件）
	TokenCount       int    `gorm:"default:0" json:"token_count"`
	AgentName        string `gorm:"size:100" json:"agent_name"`
}

func (ChatMessage) TableName() string { return "chat_messages" }

// Attachment 消息附件（图片/文件）
type Attachment struct {
	Path      string `json:"path"`       // 相对于工作区的路径
	Name      string `json:"name"`       // 原始文件名
	MediaType string `json:"media_type"` // MIME 类型
	Size      int64  `json:"size"`       // 文件大小（字节）
}
