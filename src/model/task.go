package model

import (
	"strconv"
	"strings"
	"time"
)

// Task 任务模型
type Task struct {
	BaseModel
	WorkspaceID    uint       `gorm:"not null;index" json:"workspace_id"`
	Workspace      Workspace  `gorm:"foreignKey:WorkspaceID" json:"workspace"`
	ConversationID uint       `gorm:"default:0;index" json:"conversation_id"`
	ParentID       uint       `gorm:"default:0;index" json:"parent_id"`
	Title          string     `gorm:"size:500;not null" json:"title" validate:"required,max=500"`
	Description    string     `gorm:"type:text" json:"description"`
	Detail         string     `json:"detail" gorm:"type:text"`
	TaskDoc        string     `json:"task_doc" gorm:"size:500"`
	DependsOn      string     `json:"depends_on" gorm:"size:500"` // 依赖的任务 ID 列表（逗号分隔，同阶段内）
	Phase          int        `gorm:"default:1" json:"phase"`
	PhaseLabel     string     `gorm:"size:100" json:"phase_label"`
	Status         int        `gorm:"default:0;index" json:"status"` // 0=待处理 1=进行中 2=已完成 3=失败 4=跳过
	Priority       int        `gorm:"default:0" json:"priority"`     // 0=普通 1=高 2=紧急
	AssigneeID     uint       `gorm:"default:0" json:"assignee_id"`
	Assignee       Agent      `gorm:"foreignKey:AssigneeID" json:"assignee"`
	Result         string     `gorm:"type:text" json:"result"`
	RequiredSkills string     `gorm:"size:500" json:"required_skills"`
	AssignReason   string     `gorm:"size:500" json:"assign_reason"`
	Sort           int        `gorm:"default:0" json:"sort"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}

func (Task) TableName() string { return "tasks" }

// TaskCreateReq 创建任务请求
type TaskCreateReq struct {
	WorkspaceID    uint   `validate:"required"`
	ConversationID uint
	ParentID       uint
	Title          string `validate:"required,max=500"`
	Description    string
	Detail         string
	TaskDoc        string
	DependsOn      string
	Phase          int
	PhaseLabel     string
	Priority       int
	AssigneeID     uint
	RequiredSkills string
	Sort           int
}

// TaskUpdateReq 更新任务请求
type TaskUpdateReq struct {
	Title          string `validate:"omitempty,max=500"`
	Description    *string
	Detail         *string
	TaskDoc        *string
	DependsOn      *string
	Status         *int `validate:"omitempty,oneof=0 1 2 3 4"`
	Priority       *int `validate:"omitempty,oneof=0 1 2"`
	AssigneeID     *uint
	RequiredSkills *string
	ConversationID *uint
	Result         *string
	PhaseLabel     *string
}

// TaskStatusLabel 任务状态文本
func TaskStatusLabel(status int) string {
	switch status {
	case 0:
		return "待处理"
	case 1:
		return "进行中"
	case 2:
		return "已完成"
	case 3:
		return "失败"
	case 4:
		return "跳过"
	default:
		return "未知"
	}
}

// BuildTaskTree 从扁平任务列表构建父子关系 map（供所有格式化函数复用）
func BuildTaskTree(tasks []Task) (roots []Task, childMap map[uint][]Task) {
	childMap = make(map[uint][]Task)
	for _, t := range tasks {
		if t.ParentID == 0 {
			roots = append(roots, t)
		} else {
			childMap[t.ParentID] = append(childMap[t.ParentID], t)
		}
	}
	return
}

// ParseDependsOn 解析逗号分隔的依赖任务 ID 字符串
func ParseDependsOn(s string) []uint {
	if s == "" {
		return nil
	}
	var ids []uint
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if id, err := strconv.ParseUint(part, 10, 64); err == nil && id > 0 {
			ids = append(ids, uint(id))
		}
	}
	return ids
}
