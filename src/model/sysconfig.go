package model

// SysConfig 系统配置键值模型
type SysConfig struct {
	Key     string `gorm:"primaryKey;size:100" json:"key"`
	Value   string `gorm:"size:500" json:"value"`
	Label   string `gorm:"size:100" json:"label"`
	Group   string `gorm:"size:50;index" json:"group"`
	Type    string `gorm:"size:20;default:text" json:"type"` // text|number|bool|select
	Options string `gorm:"size:500" json:"options"`          // select 类型的选项 JSON
	Sort    int    `gorm:"default:0" json:"sort"`
}

func (SysConfig) TableName() string { return "sys_configs" }
