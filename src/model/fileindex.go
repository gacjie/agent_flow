package model

// FileIndex 文件索引模型
// 用于增量索引工作区文件，提取关键词实现按需加载上下文
type FileIndex struct {
	BaseModel
	WorkspaceID uint   `gorm:"not null;index" json:"workspace_id"`
	FilePath    string `gorm:"size:1000;not null;index" json:"file_path"` // 相对路径
	FileHash    string `gorm:"size:64;not null" json:"file_hash"`        // SHA256 指纹
	FileSize    int64  `gorm:"default:0" json:"file_size"`               // 文件大小(bytes)
	Language    string `gorm:"size:50" json:"language"`                  // 编程语言
	Keywords    string `gorm:"type:text" json:"keywords"`               // 逗号分隔的关键词
	Summary     string `gorm:"type:text" json:"summary"`                // 文件摘要
	Symbols     string `gorm:"type:text" json:"symbols"`                // 导出符号（函数/结构体名）
}

func (FileIndex) TableName() string { return "file_indexes" }
