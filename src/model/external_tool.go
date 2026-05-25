package model

// ExternalTool 外部工具注册模型
type ExternalTool struct {
	BaseModel
	Name        string `gorm:"size:100;uniqueIndex;not null" json:"name"`  // 工具唯一标识
	Label       string `gorm:"size:100;not null" json:"label"`             // 显示名称
	Description string `gorm:"size:1000" json:"description"`               // 功能描述（给LLM看）
	BinaryPath  string `gorm:"size:500;not null" json:"binary_path"`       // 二进制文件相对路径
	Parameters  string `gorm:"type:text" json:"parameters"`                // JSON Schema 参数定义
	Category    string `gorm:"size:50;index" json:"category"`              // 分类
	Version     string `gorm:"size:20" json:"version"`                     // 版本号
	Timeout     int    `gorm:"default:30" json:"timeout"`                  // 执行超时（秒）
	Protocol    string `gorm:"size:20;default:stdio" json:"protocol"`      // 通信协议：stdio
	IsSystem    bool   `gorm:"default:false" json:"is_system"`             // 是否系统内置
	Status      int    `gorm:"default:1" json:"status"`                    // 1=启用 0=禁用
	Sort        int    `gorm:"default:0" json:"sort"`
}

func (ExternalTool) TableName() string { return "external_tools" }
