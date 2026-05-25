package database

import (
	"fmt"
	"log/slog"
	"time"

	"agent_flow/src/config"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

// Init 初始化数据库连接
func Init(cfg *config.DatabaseConfig) error {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "sqlite":
		dialector = sqlite.Open(cfg.DSN)
	default:
		return fmt.Errorf("不支持的数据库驱动: %s", cfg.Driver)
	}

	// GORM 日志级别
	logLevel := logger.Warn
	if config.Get().Server.Mode == "debug" {
		logLevel = logger.Info
	}

	var err error
	db, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	// 连接池配置
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("获取底层数据库连接失败: %w", err)
	}

	// SQLite 文件锁限制，强制单连接
	if cfg.Driver == "sqlite" {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	} else {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	sqlDB.SetConnMaxLifetime(time.Hour)

	// SQLite 性能优化
	if cfg.Driver == "sqlite" {
		OptimizeSQLite(db)
	}

	slog.Info("数据库连接成功", "driver", cfg.Driver)
	return nil
}

// OptimizeSQLite 为 SQLite 连接设置性能 PRAGMA（可被其他包复用）
func OptimizeSQLite(db *gorm.DB) {
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")
	db.Exec("PRAGMA busy_timeout=5000")
	db.Exec("PRAGMA cache_size=-8000") // 8MB
}

// Get 获取数据库实例
func Get() *gorm.DB {
	return db
}

// AutoMigrate 自动迁移数据表
func AutoMigrate(models ...interface{}) error {
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	slog.Info("数据库迁移完成")
	return nil
}

// Close 关闭数据库连接
func Close() error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
