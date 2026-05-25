package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"agent_flow/src/common"
	"agent_flow/src/model"

	"github.com/go-chi/chi/v5"
)

// ListConversations 获取会话列表（JSON）
func (c *WorkbenchCtrl) ListConversations(w http.ResponseWriter, r *http.Request) {
	wsIDStr := r.URL.Query().Get("workspace_id")
	if wsIDStr == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 不能为空")
		return
	}

	wsID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工作区 ID")
		return
	}

	// 获取工作区以取得 UUID
	ws, err := c.WorkspaceService.GetByID(uint(wsID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	sessionSvc := c.ChatService.ForWorkspace(ws.UUID)

	convType := r.URL.Query().Get("type")
	parentIDStr := r.URL.Query().Get("parent_id")

	query := sessionSvc.DB.Model(&model.Conversation{}).
		Where("workspace_id = ?", uint(wsID))

	// 按类型筛选
	switch convType {
	case "main":
		query = query.Where("conv_type = ?", "main")
	case "sub":
		query = query.Where("conv_type = ?", "sub")
	}

	// 按父会话筛选
	if parentIDStr != "" {
		parentID, err := strconv.ParseUint(parentIDStr, 10, 64)
		if err == nil {
			query = query.Where("parent_id = ?", uint(parentID))
		}
	}

	var conversations []model.Conversation
	if err := query.Order("id ASC").Find(&conversations).Error; err != nil {
		common.JSONError(w, http.StatusInternalServerError, "获取会话列表失败")
		return
	}

	common.JSONSuccess(w, conversations)
}

// CreateConversation 创建新会话（JSON）
func (c *WorkbenchCtrl) CreateConversation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WorkspaceID uint   `json:"workspace_id"`
		AgentID     uint   `json:"agent_id"`
		Title       string `json:"title"`
		ConvType    string `json:"conv_type"`
		ParentID    uint   `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.JSONError(w, http.StatusBadRequest, "参数解析失败")
		return
	}

	if req.WorkspaceID == 0 {
		common.JSONError(w, http.StatusBadRequest, "请选择工作区")
		return
	}

	// 如果未指定 Agent，使用工作区绑定的 Agent
	ws, err := c.WorkspaceService.GetByID(req.WorkspaceID)
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}
	if req.AgentID == 0 {
		req.AgentID = ws.AgentID
	}

	if req.Title == "" {
		req.Title = "新对话"
	}

	sessionSvc := c.ChatService.ForWorkspace(ws.UUID)
	conv, err := sessionSvc.CreateConversation(req.WorkspaceID, req.AgentID, req.Title)
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "创建会话失败")
		return
	}

	// 设置会话类型和父会话
	if req.ConvType != "" || req.ParentID > 0 {
		updates := map[string]interface{}{}
		if req.ConvType != "" {
			updates["conv_type"] = req.ConvType
		}
		if req.ParentID > 0 {
			updates["parent_id"] = req.ParentID
		}
		sessionSvc.DB.Model(conv).Updates(updates)
		conv.ConvType = req.ConvType
		conv.ParentID = req.ParentID
	}

	common.JSONCreated(w, conv)
}

// GetMessages 获取会话消息（JSON）
func (c *WorkbenchCtrl) GetMessages(w http.ResponseWriter, r *http.Request) {
	convID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的会话 ID")
		return
	}

	// 需要 workspace_id 来定位 session DB
	wsIDStr := r.URL.Query().Get("workspace_id")
	if wsIDStr == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 不能为空")
		return
	}
	wsID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工作区 ID")
		return
	}
	ws, err := c.WorkspaceService.GetByID(uint(wsID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	sessionSvc := c.ChatService.ForWorkspace(ws.UUID)
	messages, err := sessionSvc.GetMessages(uint(convID))
	if err != nil {
		common.JSONError(w, http.StatusInternalServerError, "获取消息失败")
		return
	}

	result := map[string]interface{}{
		"messages": messages,
	}
	if conv, convErr := sessionSvc.GetConversation(uint(convID)); convErr == nil {
		if conv.Summary != "" {
			result["summary"] = conv.Summary
			result["status"] = conv.Status
		}
		result["total_tokens"] = conv.TotalTokens
	}

	common.JSONSuccess(w, result)
}

// DeleteConversation 删除会话（JSON）
func (c *WorkbenchCtrl) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	convID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的会话 ID")
		return
	}

	// 需要 workspace_id 来定位 session DB
	wsIDStr := r.URL.Query().Get("workspace_id")
	if wsIDStr == "" {
		common.JSONError(w, http.StatusBadRequest, "workspace_id 不能为空")
		return
	}
	wsID, err := strconv.ParseUint(wsIDStr, 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的工作区 ID")
		return
	}
	ws, err := c.WorkspaceService.GetByID(uint(wsID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "工作区不存在")
		return
	}

	sessionSvc := c.ChatService.ForWorkspace(ws.UUID)
	if err := sessionSvc.DeleteConversation(uint(convID)); err != nil {
		common.JSONError(w, http.StatusInternalServerError, "删除失败")
		return
	}

	common.JSONSuccess(w, map[string]string{"message": "已删除"})
}
