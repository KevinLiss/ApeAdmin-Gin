package core

import (
	"fmt"
	"log"
	"os"
	"time"

	"gin-apeadmin/internal/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB 初始化数据库连接
// models 参数由 bootstrap 层传入，避免 core → model 循环依赖
func InitDB(cfg *config.DatabaseConfig, models []interface{}) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	switch cfg.Type {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(parseLogLevel(cfg.LogLevel)),
		})
	case "sqlite":
		dbFile := cfg.DBName + ".db"
		db, err = gorm.Open(sqlite.Open(dbFile), &gorm.Config{
			Logger: logger.Default.LogMode(parseLogLevel(cfg.LogLevel)),
		})
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", cfg.Type)
	}
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 连接池配置
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移
	if cfg.AutoMigrate {
		// 生产环境熔断检测（详见 14.5）
		if cfg.Type == "mysql" {
			allowUnsafe := os.Getenv("GA_DATABASE_ALLOW_UNSAFE_MIGRATE")
			if allowUnsafe != "1" {
				log.Printf("[WARN] MySQL 环境开启了 auto_migrate，生产环境请使用 golang-migrate。" +
					"如确需开启，请设置 GA_DATABASE_ALLOW_UNSAFE_MIGRATE=1")
			}
		}
		if err := db.AutoMigrate(models...); err != nil {
			return nil, fmt.Errorf("自动迁移失败: %w", err)
		}
	}

	return db, nil
}

// CloseDB 关闭数据库连接
func CloseDB(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}
}

func parseLogLevel(level string) logger.LogLevel {
	switch level {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}
