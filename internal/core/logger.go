package core

import (
	"gin-apeadmin/internal/config"

	"go.uber.org/zap"
)

// InitLogger 初始化 Zap 日志
func InitLogger(cfg config.LogConfig) *zap.Logger {
	var zapCfg zap.Config
	if cfg.Format == "console" {
		zapCfg = zap.NewDevelopmentConfig()
	} else {
		zapCfg = zap.NewProductionConfig()
	}

	switch cfg.Level {
	case "debug":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapCfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	}

	logger, err := zapCfg.Build()
	if err != nil {
		panic("初始化日志失败: " + err.Error())
	}
	zap.ReplaceGlobals(logger)
	return logger
}
