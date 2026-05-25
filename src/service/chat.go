package service

import (
	"encoding/json"
	"log/slog"
	"time"

	"agent_flow/src/common"
	"agent_flow/src/provider"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

// ChatService 对话/会话业务逻辑
type ChatService struct {
	DB         *gorm.DB
	WorkingMgr *WorkingDBManager
}

// NewChatService 创建对话服务
func NewChatService(db *gorm.DB, workingMgr *WorkingDBManager) *ChatService {
	return &ChatService{DB: db, WorkingMgr: workingMgr}
}

// ForWorkspace 返回使用指定工作区 working.db 的 ChatService
// 如果 UUID 为空或 WorkingMgr 未设置，回退到主 DB
func (s *ChatService) ForWorkspace(workspaceUUID string) *ChatService {
	if s.WorkingMgr == nil || workspaceUUID == "" {
		return s
	}
	db, err := s.WorkingMgr.GetDB(workspaceUUID)
	if err != nil {
		return s
	}
	return &ChatService{DB: db, WorkingMgr: s.WorkingMgr}
}

// ========== Conversation 方法 ==========

// CreateConversation 创建会话
func (s *ChatService) CreateConversation(workspaceID, agentID uint, title string) (*model.Conversation, error) {
	uuid, _ := generateConvUUID()
	now := time.Now()
	conv := &model.Conversation{
		UUID:        uuid,
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		Title:       title,
		Status:      1,
		StartedAt:   &now,
	}
	if err := s.DB.Create(conv).Error; err != nil {
		return nil, err
	}
	return conv, nil
}

// GetConversation 获取会话详情
func (s *ChatService) GetConversation(id uint) (*model.Conversation, error) {
	var conv model.Conversation
	err := s.DB.First(&conv, id).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// ListConversations 获取工作区下的会话列表
func (s *ChatService) ListConversations(workspaceID uint, page, pageSize int) ([]model.Conversation, int64, error) {
	var conversations []model.Conversation
	var total int64

	query := s.DB.Model(&model.Conversation{}).Where("workspace_id = ?", workspaceID)
	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).Order("id ASC").Find(&conversations).Error
	return conversations, total, err
}

// CreateSubConversation 创建子会话（call_agent 工具调用时创建）
func (s *ChatService) CreateSubConversation(workspaceID, agentID, parentID uint, agentName, title string) (*model.Conversation, error) {
	uuid, _ := generateConvUUID()
	now := time.Now()
	conv := &model.Conversation{
		UUID:        uuid,
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		AgentName:   agentName,
		ParentID:    parentID,
		ConvType:    "sub",
		Title:       title,
		Status:      1,
		StartedAt:   &now,
	}
	if err := s.DB.Create(conv).Error; err != nil {
		return nil, err
	}
	return conv, nil
}

// FinishConversation 结束会话
func (s *ChatService) FinishConversation(id uint, summary string) error {
	now := time.Now()
	return s.DB.Model(&model.Conversation{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      2,
		"summary":     summary,
		"finished_at": &now,
	}).Error
}

// CancelConversation 取消会话
func (s *ChatService) CancelConversation(id uint) error {
	return s.DB.Model(&model.Conversation{}).Where("id = ?", id).Update("status", 3).Error
}

// UpdateTokens 更新 token 用量（直接赋值，反映当前上下文实际占用）
func (s *ChatService) UpdateTokens(id uint, tokens int) error {
	return s.DB.Model(&model.Conversation{}).Where("id = ?", id).
		UpdateColumn("total_tokens", tokens).Error
}

// DeleteConversation 删除会话（同时删除消息）
func (s *ChatService) DeleteConversation(id uint) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", id).Delete(&model.ChatMessage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Conversation{}, id).Error
	})
}

// ========== Message 方法 ==========

// SaveMessage 保存单条消息
func (s *ChatService) SaveMessage(conversationID uint, role, content string, toolCalls []provider.ToolCall, toolCallID string, reasoningContent string, tokenCount int, agentName string) (*model.ChatMessage, error) {
	msg := &model.ChatMessage{
		ConversationID:   conversationID,
		Role:             role,
		Content:          content,
		ReasoningContent: reasoningContent,
		ToolCallID:       toolCallID,
		TokenCount:       tokenCount,
		AgentName:        agentName,
	}

	if len(toolCalls) > 0 {
		data, _ := json.Marshal(toolCalls)
		msg.ToolCalls = string(data)
	}

	if err := s.DB.Create(msg).Error; err != nil {
		return nil, err
	}
	return msg, nil
}

// SaveMessages 批量保存消息（单事务）
func (s *ChatService) SaveMessages(msgs []*model.ChatMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	return s.DB.Create(&msgs).Error
}

// GetMessages 获取会话的所有消息
func (s *ChatService) GetMessages(conversationID uint) ([]model.ChatMessage, error) {
	var messages []model.ChatMessage
	err := s.DB.Where("conversation_id = ?", conversationID).Order("id ASC").Find(&messages).Error
	return messages, err
}

// GetMessagesAsLLM 获取消息并转换为 LLM 格式
func (s *ChatService) GetMessagesAsLLM(conversationID uint) ([]provider.Message, error) {
	messages, err := s.GetMessages(conversationID)
	if err != nil {
		return nil, err
	}

	var result []provider.Message
	for _, msg := range messages {
		m := provider.Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
			ToolCallID:       msg.ToolCallID,
		}
		if msg.ToolCalls != "" {
			_ = json.Unmarshal([]byte(msg.ToolCalls), &m.ToolCalls)
		}
		result = append(result, m)
	}

	sanitizeOrphanedToolCalls(result, conversationID)

	return result, nil
}

// sanitizeOrphanedToolCalls 检测并清除孤立的 tool_calls（会话中断时 assistant 消息已保存但 tool 响应未保存）
func sanitizeOrphanedToolCalls(messages []provider.Message, conversationID uint) {
	respondedIDs := make(map[string]struct{})
	for i := range messages {
		if messages[i].Role == "tool" && messages[i].ToolCallID != "" {
			respondedIDs[messages[i].ToolCallID] = struct{}{}
		}
	}

	for i := range messages {
		if messages[i].Role != "assistant" || len(messages[i].ToolCalls) == 0 {
			continue
		}
		var orphanedIDs []string
		for _, tc := range messages[i].ToolCalls {
			if _, ok := respondedIDs[tc.ID]; !ok {
				orphanedIDs = append(orphanedIDs, tc.ID)
			}
		}
		if len(orphanedIDs) > 0 {
			slog.Warn("检测到孤立的 tool_calls，已从消息中移除",
				"conversation_id", conversationID,
				"orphaned_ids", orphanedIDs,
				"total_tool_calls", len(messages[i].ToolCalls),
			)
			messages[i].ToolCalls = nil
		}
	}
}

// CountMessages 获取会话消息数
func (s *ChatService) CountMessages(conversationID uint) int64 {
	var count int64
	s.DB.Model(&model.ChatMessage{}).Where("conversation_id = ?", conversationID).Count(&count)
	return count
}

// UpdateConversationTitle 更新会话标题
func (s *ChatService) UpdateConversationTitle(id uint, title string) error {
	return s.DB.Model(&model.Conversation{}).Where("id = ?", id).Update("title", title).Error
}

// UpdateConversationAgent 更新会话绑定的智能体
func (s *ChatService) UpdateConversationAgent(id uint, agentID uint) error {
	return s.DB.Model(&model.Conversation{}).Where("id = ?", id).Update("agent_id", agentID).Error
}

// generateConvUUID 生成会话 UUID
func generateConvUUID() (string, error) {
	return common.GenerateUUID()
}
