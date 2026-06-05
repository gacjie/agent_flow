package model

import "time"

// APIToken 移动端 APP 使用的 API Token（无 IP 绑定）
type APIToken struct {
	Token     string    `gorm:"primaryKey;size:128" json:"token"`
	AdminID   uint      `gorm:"index;not null" json:"admin_id"`
	Name      string    `gorm:"size:100" json:"name"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (APIToken) TableName() string { return "api_tokens" }

func (t *APIToken) IsExpired() bool { return time.Now().After(t.ExpiresAt) }
