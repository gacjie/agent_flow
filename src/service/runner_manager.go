package service

import (
	"context"
	"fmt"
	"sync"
)

const runnerBufCap = 200

// RunnerEvent 是通过事件流发布的带序号事件
type RunnerEvent struct {
	Seq   int
	Event StreamEvent
}

// runSession 代表一个活跃的会话后台执行实例
type runSession struct {
	mu       sync.Mutex
	cancel   func()
	buf      [runnerBufCap]RunnerEvent // 环形事件缓冲区
	bufHead  int                       // 下一个写入位置（0-based, mod cap）
	bufSize  int                       // 当前有效事件数（0..cap）
	nextSeq  int                       // 下一个序号（单调递增，从 0 开始）
	subs     map[string]chan RunnerEvent
	finished bool
}

// push 将事件写入环形缓冲区并分发给所有订阅者（必须持有 mu 锁调用）
func (s *runSession) push(event StreamEvent) {
	re := RunnerEvent{Seq: s.nextSeq, Event: event}
	s.buf[s.bufHead] = re
	s.bufHead = (s.bufHead + 1) % runnerBufCap
	if s.bufSize < runnerBufCap {
		s.bufSize++
	}
	s.nextSeq++
	// 非阻塞分发给所有订阅者
	for _, ch := range s.subs {
		select {
		case ch <- re:
		default:
			// 订阅者消费过慢，跳过（不阻塞 fan-out）
		}
	}
}

// replayFrom 返回从 fromSeq 开始的历史事件切片（必须持有 mu 锁调用）
func (s *runSession) replayFrom(fromSeq int) []RunnerEvent {
	if s.bufSize == 0 {
		return nil
	}
	oldestSeq := s.nextSeq - s.bufSize
	if fromSeq < oldestSeq {
		fromSeq = oldestSeq
	}
	if fromSeq >= s.nextSeq {
		return nil
	}
	count := s.nextSeq - fromSeq
	result := make([]RunnerEvent, count)
	oldestPos := (s.bufHead - s.bufSize + runnerBufCap*2) % runnerBufCap
	for i := 0; i < count; i++ {
		pos := (oldestPos + (fromSeq - oldestSeq) + i) % runnerBufCap
		result[i] = s.buf[pos]
	}
	return result
}

// RunnerManager 管理后台运行的 ChatRunner 实例，独立于 HTTP 连接生命周期
type RunnerManager struct {
	mu       sync.RWMutex
	sessions map[string]*runSession // key = "workspaceUUID:convID"
}

// NewRunnerManager 创建新的 RunnerManager
func NewRunnerManager() *RunnerManager {
	return &RunnerManager{
		sessions: make(map[string]*runSession),
	}
}

// SessionKey 生成工作区+会话的唯一标识（避免不同工作区 convID 自增冲突）
func SessionKey(workspaceUUID string, convID uint) string {
	return fmt.Sprintf("%s:%d", workspaceUUID, convID)
}

// IsRunning 检查指定会话是否有活跃的后台 runner
func (m *RunnerManager) IsRunning(key string) bool {
	m.mu.RLock()
	sess, ok := m.sessions[key]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return !sess.finished
}

// IsWorkspaceRunning 检查指定工作区是否有任何活跃的后台 runner
func (m *RunnerManager) IsWorkspaceRunning(workspaceUUID string) bool {
	prefix := workspaceUUID + ":"
	m.mu.RLock()
	defer m.mu.RUnlock()
	for key, sess := range m.sessions {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			sess.mu.Lock()
			running := !sess.finished
			sess.mu.Unlock()
			if running {
				return true
			}
		}
	}
	return false
}

// Start 以独立 context 启动 ChatRunner，将事件写入缓冲区并分发给订阅者
// 若该会话已有活跃 runner，直接返回（防重复启动）
func (m *RunnerManager) Start(key string, runner *ChatRunner, cfg RunConfig) {
	if m.IsRunning(key) {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	internalCh := make(chan StreamEvent, 100)

	sess := &runSession{
		cancel: cancel,
		subs:   make(map[string]chan RunnerEvent),
	}

	m.mu.Lock()
	m.sessions[key] = sess
	m.mu.Unlock()

	// 启动 ChatRunner 后台协程
	go runner.Run(ctx, cfg, internalCh)

	// fan-out 协程：读取 internalCh，写入缓冲区并分发给订阅者
	go func() {
		defer func() {
			sess.mu.Lock()
			sess.finished = true
			sess.cancel() // 释放 context 资源
			for _, ch := range sess.subs {
				close(ch)
			}
			sess.subs = make(map[string]chan RunnerEvent)
			sess.mu.Unlock()
		}()
		for event := range internalCh {
			sess.mu.Lock()
			sess.push(event)
			sess.mu.Unlock()
		}
	}()
}

// SubscribeWithReplay 原子地订阅事件流并获取历史回放事件
// 在同一锁内完成"注册订阅"和"获取历史"，避免竞态条件
//
// 参数：
//   - convID: 会话 ID
//   - subID:  订阅者唯一标识（由调用方生成）
//   - fromSeq: 从该序号开始回放（0 = 从最早可用事件）
//
// 返回：
//   - chan RunnerEvent: 后续实时事件（finished 时已关闭的 channel）
//   - []RunnerEvent:   fromSeq 到订阅时刻之间的历史事件
func (m *RunnerManager) SubscribeWithReplay(key string, subID string, fromSeq int) (chan RunnerEvent, []RunnerEvent) {
	m.mu.RLock()
	sess, ok := m.sessions[key]
	m.mu.RUnlock()

	ch := make(chan RunnerEvent, 100)
	if !ok {
		close(ch)
		return ch, nil
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	// 在同一锁内获取历史事件
	replayed := sess.replayFrom(fromSeq)

	if sess.finished {
		close(ch)
		return ch, replayed
	}

	sess.subs[subID] = ch
	return ch, replayed
}

// Unsubscribe 取消订阅（不取消后台 runner，不关闭 channel）
// channel 失去引用后会被 GC 自动回收
func (m *RunnerManager) Unsubscribe(key string, subID string) {
	m.mu.RLock()
	sess, ok := m.sessions[key]
	m.mu.RUnlock()
	if !ok {
		return
	}
	sess.mu.Lock()
	delete(sess.subs, subID)
	sess.mu.Unlock()
}

// Publish 向指定会话注入一个外部事件（如 InvokeSubAgent 推送子会话状态）
func (m *RunnerManager) Publish(key string, event StreamEvent) {
	m.mu.RLock()
	sess, ok := m.sessions[key]
	m.mu.RUnlock()
	if !ok {
		return
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.finished {
		return
	}
	sess.push(event)
}

// Stop 强制停止指定会话的后台 runner（外部管理接口）
// 在 cancel 之前推送 stopped 事件，确保订阅者能收到停止通知
func (m *RunnerManager) Stop(key string) {
	m.mu.RLock()
	sess, ok := m.sessions[key]
	m.mu.RUnlock()
	if !ok {
		return
	}
	sess.mu.Lock()
	if !sess.finished {
		sess.push(StreamEvent{Type: "stopped", Content: "会话已被用户停止"})
		sess.cancel()
	}
	sess.mu.Unlock()
}

// ForceFinish 强制标记 session 为已完成并关闭所有订阅者
// 用于停止通过 RegisterSession 注册的被动 session（如子会话），其 cancel 是空函数
func (m *RunnerManager) ForceFinish(key string) {
	m.mu.RLock()
	sess, ok := m.sessions[key]
	m.mu.RUnlock()
	if !ok {
		return
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.finished {
		return
	}
	sess.push(StreamEvent{Type: "stopped", Content: "会话已被用户停止"})
	sess.finished = true
	sess.cancel()
	for _, ch := range sess.subs {
		close(ch)
	}
	sess.subs = make(map[string]chan RunnerEvent)
}

// SwitchConversation 将 session 从旧 key 迁移到新 key（上下文整理接力时使用）
// 订阅者 channel 不受影响，前端重连时用新 key 即可找到运行中的 session
func (m *RunnerManager) SwitchConversation(oldKey, newKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.sessions[oldKey]; ok {
		delete(m.sessions, oldKey)
		m.sessions[newKey] = sess
	}
}

// RegisterSession 创建一个被动 session（不启动 runner goroutine），用于子会话等外部驱动的场景。
// 调用方负责通过 Publish 推送事件，完成后调用返回的 cleanup 函数标记结束并关闭订阅者。
func (m *RunnerManager) RegisterSession(key string) func() {
	sess := &runSession{
		cancel: func() {},
		subs:   make(map[string]chan RunnerEvent),
	}
	m.mu.Lock()
	m.sessions[key] = sess
	m.mu.Unlock()
	return func() {
		sess.mu.Lock()
		sess.finished = true
		for _, ch := range sess.subs {
			close(ch)
		}
		sess.subs = make(map[string]chan RunnerEvent)
		sess.mu.Unlock()
	}
}

// WaitForCompletion 阻塞等待指定会话的 runner 完成
// 通过订阅 channel 等待关闭，不消耗 CPU。session 不存在或已完成时立即返回。
func (m *RunnerManager) WaitForCompletion(key string) {
	subID := "wait-completion-" + key
	ch, _ := m.SubscribeWithReplay(key, subID, 0)
	for range ch {
	}
}

// GetCurrentSeq 返回指定会话的当前事件序号（用于前端跳过已有事件）
func (m *RunnerManager) GetCurrentSeq(key string) int {
	m.mu.RLock()
	sess, ok := m.sessions[key]
	m.mu.RUnlock()
	if !ok {
		return 0
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	return sess.nextSeq
}
