package model

import "strings"

// LLMModel LLM 模型配置（单表，包含调用所需全部信息）
type LLMModel struct {
	BaseModel
	ModelID         string `gorm:"size:100;index;not null" json:"model_id"`           // 本系统标识（允许多条同 model_id 实现负载均衡）
	Name            string `gorm:"size:100;not null" json:"name"`                     // 前端显示名称
	BaseURL         string `gorm:"size:500;not null" json:"base_url"`                  // 上游请求地址
	Protocol        string `gorm:"size:20;not null" json:"protocol"`                   // openai / openai-response / anthropic / gemini
	APIKey          string `gorm:"size:500;not null" json:"-"`                         // 加密存储，序列化时隐藏
	APIModelID      string `gorm:"size:100;not null" json:"api_model_id"`              // 上游请求模型 ID
	MaxInputTokens  int    `gorm:"default:200000" json:"max_input_tokens"`             // 上下文窗口大小（输入 token 上限）
	MaxOutputTokens int    `gorm:"default:128000" json:"max_output_tokens"`            // 单次回复最大输出 token
	IsAuto          bool   `gorm:"default:false" json:"is_auto"`                       // 参与自动切换（报错时切到下一个）
	Capabilities    string `gorm:"size:200;default:'tools'" json:"capabilities"`       // 逗号分隔能力标记：vision,image_gen,tools
	ReasoningEffort string `gorm:"size:20" json:"reasoning_effort"`                    // 思考等级：空=不启用，low/medium/high/xhigh/max=启用
}

// HasCapability 检查模型是否具有指定能力
func (m *LLMModel) HasCapability(cap string) bool {
	for _, c := range strings.Split(m.Capabilities, ",") {
		if strings.TrimSpace(c) == cap {
			return true
		}
	}
	return false
}

func (LLMModel) TableName() string { return "llm_models" }

// LLMModelCreateReq 创建模型请求
type LLMModelCreateReq struct {
	ModelID         string `json:"model_id" validate:"required,max=100"`
	Name            string `json:"name" validate:"required,max=100"`
	BaseURL         string `json:"base_url" validate:"required"`
	Protocol        string `json:"protocol" validate:"required,oneof=openai openai-response anthropic gemini"`
	APIKey          string `json:"api_key" validate:"required"`
	APIModelID      string `json:"api_model_id" validate:"required,max=100"`
	MaxInputTokens  int    `json:"max_input_tokens"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	IsAuto          bool   `json:"is_auto"`
	Capabilities    string `json:"capabilities"`
	ReasoningEffort string `json:"reasoning_effort"`
}

// LLMModelExport 模型导出/导入传输格式
type LLMModelExport struct {
	ModelID         string `json:"model_id"`
	Name            string `json:"name"`
	BaseURL         string `json:"base_url"`
	Protocol        string `json:"protocol"`
	APIKey          string `json:"api_key"`
	APIModelID      string `json:"api_model_id"`
	MaxInputTokens  int    `json:"max_input_tokens"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	IsAuto          bool   `json:"is_auto"`
	Capabilities    string `json:"capabilities"`
	ReasoningEffort string `json:"reasoning_effort"`
}

// LLMModelUpdateReq 更新模型请求
type LLMModelUpdateReq struct {
	Name            string `json:"name" validate:"omitempty,max=100"`
	BaseURL         string `json:"base_url" validate:"omitempty"`
	Protocol        string `json:"protocol" validate:"omitempty,oneof=openai openai-response anthropic gemini"`
	APIKey          string `json:"api_key" validate:"omitempty"`
	APIModelID      string `json:"api_model_id" validate:"omitempty,max=100"`
	MaxInputTokens  int    `json:"max_input_tokens"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	IsAuto          *bool  `json:"is_auto"`
	Capabilities    string `json:"capabilities"`
	ReasoningEffort string `json:"reasoning_effort"`
}
