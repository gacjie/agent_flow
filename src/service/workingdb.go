package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"agent_flow/src/database"
	"agent_flow/src/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// WorkingDBManager 管理每个工作区的独立 SQLite 工作数据库
// 每个工作区在 working/{uuid}/working.db 中存储对话、消息和任务数据
type WorkingDBManager struct {
	WorkingRoot string
	dbs         map[string]*gorm.DB
	mu          sync.RWMutex
}

// NewWorkingDBManager 创建 WorkingDBManager
func NewWorkingDBManager(workingRoot string) *WorkingDBManager {
	return &WorkingDBManager{
		WorkingRoot: workingRoot,
		dbs:         make(map[string]*gorm.DB),
	}
}

// GetDB 获取指定工作区的数据库连接（如不存在则创建）
func (m *WorkingDBManager) GetDB(workspaceUUID string) (*gorm.DB, error) {
	if workspaceUUID == "" {
		return nil, fmt.Errorf("workspaceUUID 不能为空")
	}

	// 先用读锁检查缓存
	m.mu.RLock()
	db, ok := m.dbs[workspaceUUID]
	m.mu.RUnlock()
	if ok {
		return db, nil
	}

	// 写锁创建连接
	m.mu.Lock()
	defer m.mu.Unlock()

	// 双重检查
	if db, ok = m.dbs[workspaceUUID]; ok {
		return db, nil
	}

	// 确保工作区目录存在
	dir := filepath.Join(m.WorkingRoot, workspaceUUID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建工作区目录失败: %w", err)
	}

	dbPath := filepath.Join(dir, "working.db")
	newDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("打开 working.db 失败 (%s): %w", dbPath, err)
	}

	// SQLite 单连接模式
	sqlDB, err := newDB.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	// SQLite 性能优化
	database.OptimizeSQLite(newDB)

	// 自动迁移 Conversation、ChatMessage、Task、TaskDoc 表
	if err := newDB.AutoMigrate(&model.Conversation{}, &model.ChatMessage{}, &model.Task{}); err != nil {
		return nil, fmt.Errorf("working.db 迁移失败: %w", err)
	}

	m.dbs[workspaceUUID] = newDB
	slog.Info("工作区 working.db 已初始化", "uuid", workspaceUUID, "path", dbPath)
	return newDB, nil
}

// CloseWorkspace 关闭并从缓存中移除指定工作区的连接
func (m *WorkingDBManager) CloseWorkspace(workspaceUUID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if db, ok := m.dbs[workspaceUUID]; ok {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
		delete(m.dbs, workspaceUUID)
	}
}

// CloseAll 关闭所有连接
func (m *WorkingDBManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for uuid, db := range m.dbs {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
		delete(m.dbs, uuid)
	}
}
