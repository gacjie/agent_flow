package model

import (
	"time"
)

// BaseModel 所有模型的基础结构（嵌入使用）
type BaseModel struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
