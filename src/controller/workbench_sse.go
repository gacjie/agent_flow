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
		http.Error(w, "会话已结束", http.StatusBadRequest)
		return
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
			WorkspaceID:    ws.ID,
			WorkspaceUUID:  ws.UUID,
			WorkDir:        workDir,
			ProjectPath:    projectPath,
			UserMessage:    req.Message,
			Attachments:    req.Attachments,
		}
		mgr.Start(sk, c.ChatRunner, runCfg)
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

	if mgr == nil || !mgr.IsRunning(sk) {
		common.JSONError(w, http.StatusBadRequest, "会话未在运行")
		return
	}

	sessionSvc := c.ChatService.ForWorkspace(ws.UUID)
	conv, err := sessionSvc.GetConversation(uint(convID))
	if err != nil {
		common.JSONError(w, http.StatusNotFound, "会话不存在")
		return
	}

	var stoppedSubs []uint
	if conv.ConvType == "main" || conv.ParentID == 0 {
		var subConvs []model.Conversation
		sessionSvc.DB.Where("parent_id = ? AND status = 1", convID).Find(&subConvs)
		for _, sub := range subConvs {
			subKey := service.SessionKey(ws.UUID, sub.ID)
			if mgr.IsRunning(subKey) {
				mgr.ForceFinish(subKey)
				stoppedSubs = append(stoppedSubs, sub.ID)
			}
		}
	}

	mgr.Stop(sk)

	slog.Info("会话已停止", "conv_id", convID, "stopped_subs", stoppedSubs)
	common.JSONSuccess(w, map[string]interface{}{
		"stopped":           true,
		"stopped_sub_convs": stoppedSubs,
	})
}
