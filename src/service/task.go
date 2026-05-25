package service

import (
	"fmt"
	"strings"
	"time"

	"agent_flow/src/model"

	"gorm.io/gorm"
)

// TaskService 任务业务逻辑（工作区级，操作 working.db 的 tasks 表）
type TaskService struct {
	workingMgr *WorkingDBManager
}

// NewTaskService 创建任务服务
func NewTaskService(workingMgr *WorkingDBManager) *TaskService {
	return &TaskService{workingMgr: workingMgr}
}

// db 获取指定工作区的 DB 连接
func (s *TaskService) db(workspaceUUID string) (*gorm.DB, error) {
	return s.workingMgr.GetDB(workspaceUUID)
}

// Create 创建任务
func (s *TaskService) Create(workspaceUUID string, req *model.TaskCreateReq) (*model.Task, error) {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return nil, err
	}

	task := &model.Task{
		WorkspaceID:    req.WorkspaceID,
		ConversationID: req.ConversationID,
		ParentID:       req.ParentID,
		Title:          req.Title,
		Description:    req.Description,
		Detail:         req.Detail,
		TaskDoc:        req.TaskDoc,
		DependsOn:      req.DependsOn,
		Phase:          req.Phase,
		PhaseLabel:     req.PhaseLabel,
		Priority:       req.Priority,
		AssigneeID:     req.AssigneeID,
		RequiredSkills: req.RequiredSkills,
		Sort:           req.Sort,
		Status:         0,
	}
	if task.Phase == 0 {
		task.Phase = 1
	}

	if err := db.Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

// GetByID 获取任务详情
func (s *TaskService) GetByID(workspaceUUID string, id uint) (*model.Task, error) {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return nil, err
	}
	var task model.Task
	if err := db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// ListByWorkspace 获取工作区的顶级任务列表（分页，不含子任务）
func (s *TaskService) ListByWorkspace(workspaceUUID string, page, pageSize int) ([]model.Task, int64, error) {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return nil, 0, err
	}

	var tasks []model.Task
	var total int64

	query := db.Model(&model.Task{}).Where("parent_id = 0")
	query.Count(&total)

	offset := (page - 1) * pageSize
	err = query.Offset(offset).Limit(pageSize).
		Order("phase ASC, sort ASC, id ASC").Find(&tasks).Error
	return tasks, total, err
}

// ListWithSubTasks 获取工作区所有任务（含子任务，按阶段排序）
func (s *TaskService) ListWithSubTasks(workspaceUUID string) ([]model.Task, error) {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return nil, err
	}
	var tasks []model.Task
	err = db.Order("phase ASC, parent_id ASC, sort ASC, id ASC").Find(&tasks).Error
	return tasks, err
}

// ListSubTasks 获取子任务
func (s *TaskService) ListSubTasks(workspaceUUID string, parentID uint) ([]model.Task, error) {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return nil, err
	}
	var tasks []model.Task
	err = db.Where("parent_id = ?", parentID).
		Order("sort ASC, id ASC").Find(&tasks).Error
	return tasks, err
}

// Update 更新任务
func (s *TaskService) Update(workspaceUUID string, id uint, req *model.TaskUpdateReq) (*model.Task, error) {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Detail != nil {
		updates["detail"] = *req.Detail
	}
	if req.TaskDoc != nil {
		updates["task_doc"] = *req.TaskDoc
	}
	if req.DependsOn != nil {
		updates["depends_on"] = *req.DependsOn
	}
	if req.Status != nil {
		// 依赖校验：设为进行中时检查依赖任务是否已完成
		if *req.Status == 1 {
			var task model.Task
			if err := db.First(&task, id).Error; err != nil {
				return nil, fmt.Errorf("任务 ID:%d 不存在", id)
			}
			if task.DependsOn != "" {
				depIDs := model.ParseDependsOn(task.DependsOn)
				for _, depID := range depIDs {
					var dep model.Task
					if err := db.First(&dep, depID).Error; err == nil && dep.Status < 2 {
						return nil, fmt.Errorf("任务 #%d 依赖的任务 #%d [%s] 尚未完成", id, depID, dep.Title)
					}
				}
			}
		}
		updates["status"] = *req.Status
		now := time.Now()
		if *req.Status == 1 {
			updates["started_at"] = &now
		}
		if *req.Status == 2 || *req.Status == 3 {
			updates["finished_at"] = &now
		}
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.AssigneeID != nil {
		updates["assignee_id"] = *req.AssigneeID
	}
	if req.RequiredSkills != nil {
		updates["required_skills"] = *req.RequiredSkills
	}
	if req.ConversationID != nil {
		updates["conversation_id"] = *req.ConversationID
	}
	if req.Result != nil {
		updates["result"] = *req.Result
	}
	if req.PhaseLabel != nil {
		updates["phase_label"] = *req.PhaseLabel
	}

	if len(updates) > 0 {
		if err := db.Model(&model.Task{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetByID(workspaceUUID, id)
}

// Subdivide 将父任务拆分为子任务（继承阶段信息）
func (s *TaskService) Subdivide(workspaceUUID string, parentID uint, reqs []*model.TaskCreateReq) ([]*model.Task, error) {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return nil, err
	}

	var parent model.Task
	if err := db.First(&parent, parentID).Error; err != nil {
		return nil, fmt.Errorf("父任务 ID:%d 不存在", parentID)
	}

	var created []*model.Task
	for i, req := range reqs {
		task := &model.Task{
			WorkspaceID:    parent.WorkspaceID,
			ParentID:       parentID,
			Title:          req.Title,
			Description:    req.Description,
			Detail:         req.Detail,
			TaskDoc:        req.TaskDoc,
			DependsOn:      req.DependsOn,
			Phase:          parent.Phase,
			PhaseLabel:     parent.PhaseLabel,
			Priority:       req.Priority,
			AssigneeID:     req.AssigneeID,
			RequiredSkills: req.RequiredSkills,
			Sort:           i,
			Status:         0,
		}
		if err := db.Create(task).Error; err != nil {
			return created, fmt.Errorf("创建子任务 '%s' 失败: %s", req.Title, err.Error())
		}
		created = append(created, task)
	}
	return created, nil
}

// DeleteWithValidation 删除任务（含进行中检查 + 级联删除子任务）
func (s *TaskService) DeleteWithValidation(workspaceUUID string, id uint) (string, int64, error) {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return "", 0, err
	}

	var task model.Task
	if err := db.First(&task, id).Error; err != nil {
		return "", 0, fmt.Errorf("任务 ID:%d 不存在", id)
	}

	if task.Status == 1 {
		return "", 0, fmt.Errorf("任务 ID:%d [%s] 状态为「进行中」，不允许删除", id, task.Title)
	}

	var subCount int64
	db.Model(&model.Task{}).Where("parent_id = ?", id).Count(&subCount)

	if subCount > 0 {
		var runningCount int64
		db.Model(&model.Task{}).Where("parent_id = ? AND status = 1", id).Count(&runningCount)
		if runningCount > 0 {
			return "", 0, fmt.Errorf("任务 ID:%d [%s] 有 %d 个子任务正在进行中，不允许删除", id, task.Title, runningCount)
		}
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if subCount > 0 {
			if err := tx.Where("parent_id = ?", id).Delete(&model.Task{}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&model.Task{}, id).Error
	})
	return task.Title, subCount, err
}

// Delete 删除任务（同时删除子任务，无校验）
func (s *TaskService) Delete(workspaceUUID string, id uint) error {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("parent_id = ?", id).Delete(&model.Task{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Task{}, id).Error
	})
}

// GetCurrentPhase 获取工作区当前阶段（第一个未完成阶段）
func (s *TaskService) GetCurrentPhase(workspaceUUID string) int {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return 0
	}
	var task model.Task
	if err := db.Where("status < 2").Order("phase ASC").First(&task).Error; err != nil {
		return 0
	}
	return task.Phase
}

// GetPhaseTasksSummary 获取指定阶段的任务摘要（注入 LLM 上下文）
// 递归展示任意深度的父子任务树
func (s *TaskService) GetPhaseTasksSummary(workspaceUUID string, phase int) string {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return ""
	}

	var allTasks []model.Task
	db.Where("phase = ?", phase).
		Order("parent_id ASC, sort ASC, id ASC").Find(&allTasks)

	if len(allTasks) == 0 {
		return ""
	}

	total := len(allTasks)
	completed := 0
	for _, t := range allTasks {
		if t.Status >= 2 {
			completed++
		}
	}

	roots, childMap := model.BuildTaskTree(allTasks)

	phaseLabel := ""
	if len(allTasks) > 0 && allTasks[0].PhaseLabel != "" {
		phaseLabel = " - " + allTasks[0].PhaseLabel
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("当前阶段：第%d阶段%s（%d/%d完成）\n", phase, phaseLabel, completed, total))

	for _, t := range roots {
		renderPhaseSummaryNode(&sb, t, childMap, 0)
	}

	return sb.String()
}

// renderPhaseSummaryNode 递归渲染阶段摘要中的任务节点
func renderPhaseSummaryNode(sb *strings.Builder, t model.Task, childMap map[uint][]model.Task, depth int) {
	indent := strings.Repeat("  ", depth)
	icon := "▶"
	if t.Status >= 2 {
		icon = "✓"
	} else if t.Status == 0 {
		icon = "·"
	}
	status := model.TaskStatusLabel(t.Status)
	docRef := ""
	if t.TaskDoc != "" {
		docRef = fmt.Sprintf(" [文档:%s]", t.TaskDoc)
	}
	depRef := ""
	if t.DependsOn != "" {
		depRef = fmt.Sprintf(" [依赖:#%s]", t.DependsOn)
	}
	sb.WriteString(fmt.Sprintf("%s%s [%s] %s (#%d)%s%s\n", indent, icon, status, t.Title, t.ID, docRef, depRef))

	if subs, ok := childMap[t.ID]; ok {
		for _, sub := range subs {
			renderPhaseSummaryNode(sb, sub, childMap, depth+1)
		}
	}
}

// CountByWorkspace 获取工作区任务统计
func (s *TaskService) CountByWorkspace(workspaceUUID string) map[string]int64 {
	stats := map[string]int64{}
	db, err := s.db(workspaceUUID)
	if err != nil {
		return stats
	}

	var total, pending, running, completed, failed int64
	db.Model(&model.Task{}).Count(&total)
	db.Model(&model.Task{}).Where("status = 0").Count(&pending)
	db.Model(&model.Task{}).Where("status = 1").Count(&running)
	db.Model(&model.Task{}).Where("status = 2").Count(&completed)
	db.Model(&model.Task{}).Where("status = 3").Count(&failed)

	stats["total"] = total
	stats["pending"] = pending
	stats["running"] = running
	stats["completed"] = completed
	stats["failed"] = failed
	return stats
}

// CountPendingInPhase 获取指定阶段未完成任务数（用于阶段切换检测）
func (s *TaskService) CountPendingInPhase(workspaceUUID string, phase int) int64 {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return 0
	}
	var count int64
	db.Model(&model.Task{}).Where("phase = ? AND status < 2", phase).Count(&count)
	return count
}

// BatchCreate 批量创建任务（由 TaskPlanner 调用）
func (s *TaskService) BatchCreate(workspaceUUID string, tasks []*model.Task) error {
	db, err := s.db(workspaceUUID)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, task := range tasks {
			if err := tx.Create(task).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
