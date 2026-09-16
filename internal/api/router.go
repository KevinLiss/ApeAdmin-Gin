package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/config"
	"apeadmin-gin/internal/middleware"
	"apeadmin-gin/internal/pkg/response"
)

// PluginRouteRegistrar 插件路由注册回调
// 由 bootstrap 注入，在所有系统路由注册完成后调用，让 L1 插件能注册自己的路由
type PluginRouteRegistrar func(public, authed *gin.RouterGroup)

// RegisterRoutes 注册全部路由
func RegisterRoutes(r *gin.Engine, cfg *config.Config, registerPlugins PluginRouteRegistrar) {
	r.GET("/health", HealthCheck)

	// 全局限流
	if cfg.Security.RateLimit.Enabled {
		r.Use(middleware.RateLimit("api", cfg.Security.RateLimit.RequestsPerMinute, time.Minute))
	}

	api := r.Group(cfg.App.APIPrefix)

	// ─── 公开路由（免登录）───
	public := api.Group("")
	{
		auth := public.Group("/auth")
		auth.POST("/login",
			middleware.LoginGuard(),
			middleware.RateLimit("login", 10, time.Minute),
			(&AuthHandler{}).Login,
		)
		public.GET("/settings/public", GetPublicSettings)
	}

	// ─── 认证路由（仅登录）───
	authed := api.Group("")
	authed.Use(middleware.JWTAuth())
	{
		auth := authed.Group("/auth")
		auth.GET("/userinfo", (&AuthHandler{}).GetUserInfo)
		auth.GET("/profile", (&AuthHandler{}).GetProfile)
		auth.PUT("/profile", (&AuthHandler{}).UpdateProfile)
		auth.PUT("/profile/password", (&AuthHandler{}).ChangePassword)
		auth.POST("/logout", (&AuthHandler{}).Logout)
		auth.POST("/refresh", (&AuthHandler{}).RefreshToken)
	}

	// ─── 权限路由 ───
	perm := api.Group("")
	perm.Use(middleware.JWTAuth())
	{
		// 用户管理
		users := perm.Group("/users")
		userH := &UserHandler{}
		users.GET("", middleware.RequirePermission("system:user:list"), userH.List)
		users.POST("", middleware.RequirePermission("system:user:add"), userH.Create)
		users.GET("/:id", middleware.RequirePermission("system:user:list"), userH.Get)
		users.PUT("/:id", middleware.RequirePermission("system:user:edit"), userH.Update)
		users.DELETE("/:id", middleware.RequirePermission("system:user:delete"), userH.Delete)
		users.PUT("/:id/reset-password", middleware.RequirePermission("system:user:reset-password"), userH.ResetPassword)

		// 角色管理
		roles := perm.Group("/roles")
		roleH := &RoleHandler{}
		roles.GET("", middleware.RequirePermission("system:role:list"), roleH.List)
		roles.POST("", middleware.RequirePermission("system:role:add"), roleH.Create)
		roles.GET("/all", roleH.ListAll)
		roles.GET("/:id", middleware.RequirePermission("system:role:list"), roleH.Get)
		roles.PUT("/:id", middleware.RequirePermission("system:role:edit"), roleH.Update)
		roles.DELETE("/:id", middleware.RequirePermission("system:role:delete"), roleH.Delete)

		// 菜单管理
		menus := perm.Group("/menus")
		menuH := &MenuHandler{}
		menus.GET("/tree", middleware.RequirePermission("system:menu:list"), menuH.Tree)
		menus.POST("", middleware.RequirePermission("system:menu:add"), menuH.Create)
		menus.PUT("/:id", middleware.RequirePermission("system:menu:edit"), menuH.Update)
		menus.DELETE("/:id", middleware.RequirePermission("system:menu:delete"), menuH.Delete)

		// 部门管理
		depts := perm.Group("/depts")
		deptH := &DeptHandler{}
		depts.GET("/tree", middleware.RequirePermission("system:dept:list"), deptH.Tree)
		depts.POST("", middleware.RequirePermission("system:dept:add"), deptH.Create)
		depts.PUT("/:id", middleware.RequirePermission("system:dept:edit"), deptH.Update)
		depts.DELETE("/:id", middleware.RequirePermission("system:dept:delete"), deptH.Delete)

		// 插件管理
		plugins := perm.Group("/plugins")
		pluginH := &PluginHandler{}
		plugins.GET("", middleware.RequirePermission("system:plugin:list"), pluginH.List)
		plugins.PUT("/:id/toggle", middleware.RequirePermission("system:plugin:toggle"), pluginH.Toggle)
		plugins.GET("/:id/config", middleware.RequirePermission("system:plugin:config"), pluginH.GetConfig)
		plugins.PUT("/:id/config", middleware.RequirePermission("system:plugin:config"), pluginH.UpdateConfig)
		plugins.POST("/upload", middleware.RequirePermission("system:plugin:upload"), pluginH.Upload)
		plugins.POST("/restart", middleware.RequirePermission("system:plugin:restart"), pluginH.Restart)
		plugins.DELETE("/:id", middleware.RequirePermission("system:plugin:delete"), pluginH.Delete)

		// 系统日志
		logs := perm.Group("/logs")
		logH := &LogHandler{}
		logs.GET("", middleware.RequirePermission("system:log:list"), logH.List)
		logs.DELETE("", middleware.RequirePermission("system:log:delete"), logH.Clear)
		logs.GET("/:id", middleware.RequirePermission("system:log:list"), logH.Get)
		logs.DELETE("/:id", middleware.RequirePermission("system:log:delete"), logH.Delete)

		// 系统设置
		settings := perm.Group("/settings")
		setH := &SettingHandler{}
		settings.GET("", middleware.RequirePermission("system:setting:list"), setH.List)
		settings.PUT("", middleware.RequirePermission("system:setting:edit"), setH.BatchUpdate)
		settings.PUT("/:key", middleware.RequirePermission("system:setting:edit"), setH.Update)

		// 仪表盘
		dashboard := perm.Group("/dashboard")
		dashH := &DashboardHandler{}
		dashboard.GET("/system", middleware.RequirePermission("dashboard:view"), dashH.System)
		dashboard.GET("/stats", middleware.RequirePermission("dashboard:view"), dashH.Stats)

		// MCP 管理
		mcpGroup := perm.Group("/mcp")
		mcpH := &McpHandler{}
		mcpGroup.GET("/tools", middleware.RequirePermission("mcp:tools:list"), mcpH.Tools)
		mcpGroup.GET("/tools/categories", middleware.RequirePermission("mcp:tools:list"), mcpH.ToolCategories)
		mcpGroup.POST("/tools/call", middleware.RequirePermission("mcp:tools:call"), mcpH.ToolCall)
		mcpGroup.GET("/resources", middleware.RequirePermission("mcp:resources:list"), mcpH.Resources)
		mcpGroup.GET("/resources/read", middleware.RequirePermission("mcp:resources:list"), mcpH.ResourceRead)
		mcpGroup.GET("/prompts", middleware.RequirePermission("mcp:prompts:list"), mcpH.Prompts)
		mcpGroup.POST("/prompts/render", middleware.RequirePermission("mcp:prompts:list"), mcpH.PromptRender)
		mcpGroup.GET("/audit-logs", middleware.RequirePermission("mcp:audit:list"), mcpH.AuditLogs)

		// AI 供应商管理
		ai := perm.Group("/ai")
		aiH := &AiHandler{}
		ai.GET("/providers", middleware.RequirePermission("ai:provider:list"), aiH.ListProviders)
		ai.POST("/providers", middleware.RequirePermission("ai:provider:add"), aiH.CreateProvider)
		ai.GET("/providers/all", middleware.RequirePermission("ai:provider:list"), aiH.ListAllProviders)
		ai.PUT("/providers/:id", middleware.RequirePermission("ai:provider:edit"), aiH.UpdateProvider)
		ai.DELETE("/providers/:id", middleware.RequirePermission("ai:provider:delete"), aiH.DeleteProvider)
		ai.POST("/providers/:id/test", middleware.RequirePermission("ai:provider:list"), aiH.TestProvider)

		// AI 对话
		ai.POST("/chat", middleware.RequirePermission("ai:chat"), aiH.Chat)
		ai.POST("/chat/stream", middleware.RequirePermission("ai:chat"), aiH.ChatStream)

		// AI 会话持久化
		ai.GET("/sessions", middleware.RequirePermission("ai:chat"), aiH.ListSessions)
		ai.POST("/sessions", middleware.RequirePermission("ai:chat"), aiH.CreateSession)
		ai.GET("/sessions/:id", middleware.RequirePermission("ai:chat"), aiH.GetSessionMessages)
		ai.PUT("/sessions/:id", middleware.RequirePermission("ai:chat"), aiH.RenameSession)
		ai.DELETE("/sessions/:id", middleware.RequirePermission("ai:chat"), aiH.DeleteSession)
		ai.POST("/sessions/:id/messages", middleware.RequirePermission("ai:chat"), aiH.AppendMessage)
	}

	// ─── 插件路由注册 ───
	// 在所有系统路由注册完成后，回调让 L1 插件注册自己的 HTTP 路由
	if registerPlugins != nil {
		registerPlugins(public, perm)
	}

	// SPA 静态文件服务
	if cfg.App.SPADir != "" {
		if _, err := os.Stat(cfg.App.SPADir); err == nil {
			r.Static("/assets", filepath.Join(cfg.App.SPADir, "assets"))
			r.StaticFile("/favicon.ico", filepath.Join(cfg.App.SPADir, "favicon.ico"))
			// SPA fallback：非 API 路径返回 index.html
			r.NoRoute(func(c *gin.Context) {
				if strings.HasPrefix(c.Request.URL.Path, cfg.App.APIPrefix) ||
					strings.HasPrefix(c.Request.URL.Path, "/health") {
					c.JSON(http.StatusNotFound, response.Error(404, "接口不存在"))
					return
				}
				c.File(filepath.Join(cfg.App.SPADir, "index.html"))
			})
		} else {
			r.NoRoute(func(c *gin.Context) {
				c.JSON(http.StatusOK, response.Success(gin.H{
					"message": "ApeAdmin-Gin API Server",
					"version": cfg.App.Version,
				}))
			})
		}
	} else {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusOK, response.Success(gin.H{
				"message": "ApeAdmin-Gin API Server",
				"version": cfg.App.Version,
			}))
		})
	}
}
