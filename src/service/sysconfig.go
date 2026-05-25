package service

import (
	"log/slog"
	"sync"

	"agent_flow/src/model"

	"gorm.io/gorm"
)

// SysConfigService 系统配置服务（含内存缓存）
type SysConfigService struct {
	DB    *gorm.DB
	mu    sync.RWMutex
	cache map[string]string // key → value
}

// NewSysConfigService 创建配置服务并加载缓存
func NewSysConfigService(db *gorm.DB) *SysConfigService {
	s := &SysConfigService{DB: db, cache: make(map[string]string)}
	s.Reload()
	return s
}

// Reload 重新加载所有配置到内存
func (s *SysConfigService) Reload() {
	var configs []model.SysConfig
	s.DB.Find(&configs)

	newCache := make(map[string]string, len(configs))
	for _, c := range configs {
		newCache[c.Key] = c.Value
	}

	s.mu.Lock()
	s.cache = newCache
	s.mu.Unlock()

	slog.Debug("系统配置缓存已刷新", "count", len(newCache))
}

// Get 获取配置值
func (s *SysConfigService) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cache[key]
}

// GetAll 获取所有配置记录
func (s *SysConfigService) GetAll() ([]model.SysConfig, error) {
	var configs []model.SysConfig
	err := s.DB.Order("`group` ASC, sort ASC").Find(&configs).Error
	return configs, err
}

// GetByGroup 按分组获取配置
func (s *SysConfigService) GetByGroup(group string) ([]model.SysConfig, error) {
	var configs []model.SysConfig
	err := s.DB.Where("`group` = ?", group).Order("sort ASC").Find(&configs).Error
	return configs, err
}

// Set 设置单个配置
func (s *SysConfigService) Set(key, value string) error {
	err := s.DB.Model(&model.SysConfig{}).Where("`key` = ?", key).Update("value", value).Error
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cache[key] = value
	s.mu.Unlock()
	return nil
}

// BatchUpdate 批量更新配置
func (s *SysConfigService) BatchUpdate(kvs map[string]string) error {
	for key, value := range kvs {
		if err := s.DB.Model(&model.SysConfig{}).Where("`key` = ?", key).Update("value", value).Error; err != nil {
			return err
		}
	}
	s.Reload()
	return nil
}
