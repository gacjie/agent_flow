package model

import "time"

// Session 管理员会话模型
type Session struct {
	Token     string    `gorm:"primaryKey;size:128" json:"token"`
	AdminID   uint      `gorm:"index;not null" json:"admin_id"`
	IP        string    `gorm:"size:45" json:"ip"`
	UserAgent string    `gorm:"size:255" json:"user_agent"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (Session) TableName() string {
	return "sessions"
}

// IsExpired 检查会话是否已过期
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
