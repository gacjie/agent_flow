package model

// SystemTool 系统内置工具（Go 实现的工具，启动时从 Registry 自动同步）
type SystemTool struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;size:100" json:"name"`
	Label       string `gorm:"size:100" json:"label"`
	Description string `gorm:"size:500" json:"description"`
	Category    string `gorm:"size:50;index" json:"category"` // 分类，仅展示用
	Status      int    `gorm:"default:1" json:"status"`       // 1=启用 0=禁用
	Sort        int    `gorm:"default:0" json:"sort"`
}
