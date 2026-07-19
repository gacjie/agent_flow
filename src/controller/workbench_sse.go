package controller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"agent_flow/src/common"
	"agent_flow/src/model"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// SendMessage SSE 流式端点 — 发送消息并获取 AI 回复（或重连到进行中的任务）
func (c *WorkbenchCtrl) SendMessage(w http.ResponseWriter, r *http.Request) {
	convID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "无效的会话 ID", http.StatusBadRequest)
		return
	}

	// 解析请求体（LastSeq 用于重连时指定回放起点，0 表示从最早可用事件开始）
	var req struct {
		Message     string             `json:"message"`
		AgentID     uint               `json:"agent_id"`
		ModelID     string             `json:"model_id"`
		WorkspaceID uint               `json:"workspace_id"`
		LastSeq     int                `json:"last_seq"`
		Attachments []model.Attachment `json:"attachments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数解析失败", http.StatusBadRequest)
		return
	}

	if req.WorkspaceID == 0 {
		http.Error(w, "workspace_id 不能为空", http.StatusBadRequest)
		return
	}
	ws, err := c.WorkspaceService.GetByID(req.WorkspaceID)
	if err != nil {
		http.Error(w, "工作区不存在", http.StatusNotFound)
		return
	}

	sessionSvc := c.ChatService.ForWorkspace(ws.UUID)

	// 获取会话信息
	conv, err := sessionSvc.GetConversation(uint(convID))
	if err != nil {
		http.Error(w, "会话不存在", http.StatusNotFound)
		return
	}
	if conv.Status != 1 {
		// 允许已取消的子会话重新继续（用户手动在子会话发消息）
		if conv.Status == 3 && conv.ConvType == "sub" {
			sessionSvc.ReopenConversation(conv.ID)
			conv.Status = 1
		} else {
			http.Error(w, "会话已结束", http.StatusBadRequest)
			return
		}
	}

	mgr := c.RunnerManager
	sk := service.SessionKey(ws.UUID, uint(convID))
	isRunning := mgr != nil && mgr.IsRunning(sk)

	// 决策：新建任务 vs 订阅已有任务
	if req.Message == "" {
		// 空消息：仅允许在已有任务运行时订阅（重连场景）
		if !isRunning {
			// 无活跃任务，返回 SSE 结束信号，避免前端出现 console 错误
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			fmt.Fprintf(w, "data: [DONE]\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}
	} else {
		// 有消息：任务运行中时拒绝重复发送
		if isRunning {
			http.Error(w, "任务正在执行中，请等待完成", http.StatusConflict)
			return
		}
	}

	// 设置 SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "不支持流式响应", http.StatusInternalServerError)
		return
	}

	// 禁用 SSE 连接的写超时
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		slog.Warn("无法禁用 SSE 写超时", "error", err)
	}

	// 确定 AgentID（前端覆盖 > 会话绑定 > 工作区绑定）
	agentID := conv.AgentID
	if req.AgentID > 0 {
		agentID = req.AgentID
	}
	if agentID == 0 {
		agentID = ws.AgentID
	}

	// 构建 subID（订阅者唯一标识）
	subID, _ := common.GenerateToken(8)

	// 启动新任务或订阅已有任务
	if mgr == nil {
		// RunnerManager 未注入，降级为直接运行（关闭页面会中断任务）
		slog.Warn("RunnerManager 未注入，使用降级模式", "conv_id", convID)
		http.Error(w, "服务未就绪，请重试", http.StatusServiceUnavailable)
		return
	}

	if !isRunning && req.Message != "" {
		// 主会话发消息前清理当前主会话关联的残留子会话
		if conv.ConvType == "main" || conv.ParentID == 0 {
			var activeSubConvs []model.Conversation
			sessionSvc.DB.Where("parent_id = ? AND status IN (1, 3)", convID).Find(&activeSubConvs)
			for _, sub := range activeSubConvs {
				subKey := service.SessionKey(ws.UUID, sub.ID)
				if mgr.IsRunning(subKey) {
					mgr.ForceFinish(subKey)
				}
				if sub.Status == 1 {
					sessionSvc.CancelConversation(sub.ID)
				}
			}
		}

		// 切换智能体时持久化到数据库，确保刷新后下拉框显示正确
		if agentID > 0 && agentID != conv.AgentID {
			sessionSvc.UpdateConversationAgent(conv.ID, agentID)
		}

		workDir := c.WorkspaceService.GetWorkDir(ws.UUID)
		var projectPath string
		if ws.ProjectID > 0 && ws.Project.Path != "" {
			projectPath = ws.Project.Path
		}
		runCfg := service.RunConfig{
			ConversationID: uint(convID),
			AgentID:        agentID,
			ModelOverride:  req.ModelID,
			WorkspaceID:    ws.ID,
			WorkspaceUUID:  ws.UUID,
			WorkDir:        workDir,
			ProjectPath:    projectPath,
			UserMessage:    req.Message,
			Attachments:    req.Attachments,
			IsSubConv:      conv.ConvType == "sub",
			ParentConvID:   conv.ParentID,
			AgentName:      conv.AgentName,
		}
		mgr.Start(sk, c.ChatRunner, runCfg)

		// 子会话手动继续：启动后台协程等待完成，自动总结并驱动主会话继续
		if conv.ConvType == "sub" && conv.ParentID > 0 {
			go c.handleSubConvCompletion(ws, conv, agentID, sk)
		}
	}

	// 原子地订阅并获取回放事件
	subCh, replayed := mgr.SubscribeWithReplay(sk, subID, req.LastSeq)

	// 推送回放历史事件
	for _, re := range replayed {
		if !c.writeSSEEvent(w, flusher, re) {
			mgr.Unsubscribe(sk, subID)
			return
		}
	}

	// 转发实时事件
	for {
		select {
		case re, ok := <-subCh:
			if !ok {
				// runner 已完成，发送结束信号
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}
			if !c.writeSSEEvent(w, flusher, re) {
				// 写入失败（客户端断开），只取消订阅，runner 继续在后台运行
				mgr.Unsubscribe(sk, subID)
				return
			}
		case <-r.Context().Done():
			// 客户端断开，只取消订阅，runner 继续在后台运行
			mgr.Unsubscribe(sk, subID)
			return
		}
	}
}

// writeSSEEvent 序列化并写入一个 SSE 事件（含序号），返回是否成功
func (c *WorkbenchCtrl) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, re service.RunnerEvent) bool {
	payload := map[string]interface{}{
		"seq":  re.Seq,
		"type": re.Event.Type,
	}
	if re.Event.Content != "" {
		payload["content"] = re.Event.Content
	}
	if re.Event.Data != nil {
		payload["data"] = re.Event.Data
	}
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("SSE 事件序列化失败", "error", err)
		return true // 序列化错误不算连接断开，继续
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		slog.Warn("SSE 写入失败（客户端已断开）", "error", err)
		return false
	}
	flusher.Flush()
	return true
}

// StopConversation 停止正在运行的会话
func (c *WorkbenchCtrl) StopConversation(w http.ResponseWriter, r *http.Request) {
	convID, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		common.JSONError(w, http.StatusBadRequest, "无效的会话 ID")
		return
	}

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

	mgr := c.RunnerManager
	sk := service.SessionKey(ws.UUID, uint(convID))
	sessionSvc := c.ChatService.ForWorkspace(ws.UUID)

	if mgr == nil {
		common.JSONError(w, http.StatusBadRequest, "会话未在运行")
		return
	}

	// 子会话自身可能已结束，但父会话仍在运行，此时也允许停止
	if !mgr.IsRunning(sk) {
		conv, _ := sessionSvc.GetConversation(uint(convID))
		if conv != nil && conv.ConvType == "sub" && conv.ParentID > 0 {
			mainKey := service.SessionKey(ws.UUID, conv.ParentID)
			if !mgr.IsRunning(mainKey) {
				common.JSONError(w, http.StatusBadRequest, "会话未在运行")
				return
			}
		} else {
			common.JSONError(w, http.StatusBadRequest, "会话未在运行")
			return
		}
	}

	conv, err := sessionSvc.GetConversation(uint(convID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "会话不存在")
		return
	}

	var stoppedSubs []uint
	var stoppedMainConvID uint
	if conv.ConvType == "main" || conv.ParentID == 0 {
		// 主会话视图：停止所有关联子会话
		var subConvs []model.Conversation
		sessionSvc.DB.Where("parent_id = ? AND status = 1", convID).Find(&subConvs)
		for _, sub := range subConvs {
			subKey := service.SessionKey(ws.UUID, sub.ID)
			if mgr.IsRunning(subKey) {
				mgr.ForceFinish(subKey)
				stoppedSubs = append(stoppedSubs, sub.ID)
			}
		}
		mgr.Stop(sk)
	} else {
		// 子会话视图：通过 parent_id 找到关联的父会话和兄弟子会话一起停止
		mainConvID := conv.ParentID
		stoppedMainConvID = mainConvID

		// 1. ForceFinish 所有同一父会话下 status=1 的子会话
		var subConvs []model.Conversation
		sessionSvc.DB.Where("parent_id = ? AND status = 1", mainConvID).Find(&subConvs)
		for _, sub := range subConvs {
			subKey := service.SessionKey(ws.UUID, sub.ID)
			if mgr.IsRunning(subKey) {
				mgr.ForceFinish(subKey)
				stoppedSubs = append(stoppedSubs, sub.ID)
			}
		}

		// 2. 确保当前子会话也被停止（可能不在上面的查询结果中）
		if mgr.IsRunning(sk) {
			mgr.ForceFinish(sk)
		}

		// 3. 停止父主会话
		mainKey := service.SessionKey(ws.UUID, mainConvID)
		if mgr.IsRunning(mainKey) {
			mgr.Stop(mainKey)
		}
	}

	slog.Info("会话已停止", "conv_id", convID, "stopped_subs", stoppedSubs, "stopped_main", stoppedMainConvID)
	common.JSONSuccess(w, map[string]interface{}{
		"stopped":              true,
		"stopped_sub_convs":    stoppedSubs,
		"stopped_main_conv_id": stoppedMainConvID,
	})
}

// handleSubConvCompletion 等待子会话 runner 完成后，更新父会话中的工具结果消息
// 全部子会话完成时自动驱动主会话继续
func (c *WorkbenchCtrl) handleSubConvCompletion(ws *model.Workspace, subConv *model.Conversation, agentID uint, subKey string) {
	mgr := c.RunnerManager

	// 等待子会话 runner 完成
	mgr.WaitForCompletion(subKey)

	sessionSvc := c.ChatService.ForWorkspace(ws.UUID)

	// 生成总结并标记完成
	subCfg := service.RunConfig{
		ConversationID: subConv.ID,
		AgentID:        agentID,
		WorkspaceUUID:  ws.UUID,
		IsSubConv:      true,
	}
	summary := c.ChatRunner.SummarizeConversation(subCfg, ws.UUID, subConv.ID)
	if summary == "" {
		summary = "子会话已完成（无总结内容）"
	}
	sessionSvc.FinishConversation(subConv.ID, summary)

	// 更新父会话中的错误 tool result 为实际总结
	if subConv.ToolCallID != "" {
		newContent := fmt.Sprintf("[%s 的回复]\n%s", subConv.Title, summary)
		sessionSvc.DB.Model(&model.ChatMessage{}).
			Where("conversation_id = ? AND role = ? AND tool_call_id = ?",
				subConv.ParentID, "tool", subConv.ToolCallID).
			Update("content", newContent)
	}

	// 通知父会话订阅者
	parentKey := service.SessionKey(ws.UUID, subConv.ParentID)
	mgr.Publish(parentKey, service.StreamEvent{
		Type: "sub_conv_done",
		Data: map[string]interface{}{"conv_id": subConv.ID},
	})

	// 检查是否还有未完成的兄弟子会话（进行中或已取消待继续）
	var pendingCount int64
	sessionSvc.DB.Model(&model.Conversation{}).
		Where("parent_id = ? AND status IN (1, 3)", subConv.ParentID).
		Count(&pendingCount)
	if pendingCount > 0 {
		slog.Info("子会话完成，等待其他兄弟子会话",
			"sub_conv_id", subConv.ID, "pending", pendingCount)
		return
	}

	// 所有子会话都已完成 → 工具结果已全部更新，自动驱动主会话
	parentConv, err := sessionSvc.GetConversation(subConv.ParentID)
	if err != nil || parentConv.Status != 1 {
		slog.Info("子会话全部完成但父会话不可继续", "parent_id", subConv.ParentID)
		return
	}
	if mgr.IsRunning(parentKey) {
		return
	}

	workDir := c.WorkspaceService.GetWorkDir(ws.UUID)
	var projectPath string
	if ws.ProjectID > 0 && ws.Project.Path != "" {
		projectPath = ws.Project.Path
	}
	mainCfg := service.RunConfig{
		ConversationID: subConv.ParentID,
		AgentID:        parentConv.AgentID,
		WorkspaceID:    ws.ID,
		WorkspaceUUID:  ws.UUID,
		WorkDir:        workDir,
		ProjectPath:    projectPath,
		UserMessage:    "",
	}
	mgr.Start(parentKey, c.ChatRunner, mainCfg)
	slog.Info("所有子会话完成，自动继续主会话", "parent_id", subConv.ParentID)
}
