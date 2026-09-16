package plugin

import (
	"context"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/mcp"
	"gorm.io/gorm"
)

// Plugin 插件接口
type Plugin interface {
	Name() string
	DisplayName() string
	Description() string
	Version() string
	Author() string
	Dependencies() []string

	OnLoad() error
	Install() error
	Register(r *PluginRouter) error
	Unregister() error
	Uninstall() error
	OnUnload()
}

// PluginRouter 插件路由注册辅助
type PluginRouter struct {
	Public *gin.RouterGroup
	Authed *gin.RouterGroup
	MCP    *mcp.Registrar
	DB     *gorm.DB
}

// McpToolHandler MCP 工具处理函数
type McpToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// EventHandler 事件处理函数
type EventHandler func(ctx context.Context, payload interface{})

// 事件常量
const (
	EventAppStartup  = "app_startup"
	EventAppShutdown = "app_shutdown"
	EventDBReady     = "db_ready"
	EventUserLogin   = "user_login"
)
