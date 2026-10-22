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

	// 语言解析（全局，后端消息翻译用）
	r.Use(middleware.Locale())

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
		// health 同时注册到 API 前缀下：前端轮询 /api/v1/health（dev 经 vite 代理、
		// 生产直连），根路径 /health 保留兼容
		public.GET("/health", HealthCheck)

		// i18n 公开接口（登录页/前端启动即用）
		i18nPub := public.Group("/i18n")
		i18nPub.GET("/locales", GetI18nLocales)
		i18nPub.GET("/messages/:locale", GetI18nMessages)
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
		settings.POST("/brand-image", middleware.RequirePermission("system:setting:edit"), setH.BrandImage)

		// 系统版本与自更新（顶栏版本更新弹窗）
		version := perm.Group("/system")
		verH := &VersionHandler{}
		version.GET("/version", middleware.RequirePermission("system:setting:list"), verH.GetVersion)
		version.POST("/update", middleware.RequirePermission("system:setting:edit"), verH.Update)

		// 多语言管理
		i18nGroup := perm.Group("/i18n")
		localeH := &LocaleHandler{}
		transH := &TranslationHandler{}
		// 语言管理（Delete 按语言 code 传，与其他模块按 id 删不同）
		// 语言管理列表（含停用；GET /i18n/locales 已被公开接口占用，公开版只回启用语言）
		i18nGroup.GET("/locales/all", middleware.RequirePermission("system:i18n:list"), localeH.List)
		i18nGroup.POST("/locales", middleware.RequirePermission("system:i18n:add"), localeH.Create)
		i18nGroup.PUT("/locales/:id", middleware.RequirePermission("system:i18n:edit"), localeH.Update)
		i18nGroup.DELETE("/locales/:code", middleware.RequirePermission("system:i18n:delete"), localeH.Delete)
		// 翻译维护
		i18nGroup.GET("/translations", middleware.RequirePermission("system:i18n:list"), transH.List)
		i18nGroup.PUT("/translations", middleware.RequirePermission("system:i18n:edit"), transH.BatchSave)
		i18nGroup.POST("/translations/import", middleware.RequirePermission("system:i18n:edit"), transH.Import)
		i18nGroup.GET("/translations/export", middleware.RequirePermission("system:i18n:list"), transH.Export)

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

	// SPA 静态文件服务（admin_path 已在 bootstrap 阶段从数据库读出覆盖，改设置 + 重启即生效）
	adminPath := cfg.App.AdminPath
	if adminPath == "" {
		adminPath = "/admin"
	}
	if cfg.App.SPADir != "" {
		if _, err := os.Stat(cfg.App.SPADir); err == nil {
			// 前端 base 为 /admin/，静态资源挂到 admin_path/assets；同时保留 /assets 兼容
			assetsDir := filepath.Join(cfg.App.SPADir, "assets")
			if adminPath != "" {
				r.Static(adminPath+"/assets", assetsDir)
			}
			r.Static("/assets", assetsDir)
			r.StaticFile("/favicon.ico", filepath.Join(cfg.App.SPADir, "favicon.ico"))
			if adminPath != "" {
				r.StaticFile(adminPath+"/favicon.ico", filepath.Join(cfg.App.SPADir, "favicon.ico"))
			}
			// 根路径重定向到后台（与 Python 版 root_redirect 对齐：读数据库 admin_path）
			r.GET("/", func(c *gin.Context) {
				c.Redirect(http.StatusFound, adminPath+"/")
			})
			// SPA fallback：非 API 路径返回 index.html
			r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, cfg.App.APIPrefix) ||
				strings.HasPrefix(c.Request.URL.Path, "/health") {
				c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "common.api_not_found", nil))
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

	// 上传文件静态托管（品牌图片等），映射到 file.storage_dir
	if cfg.File.StorageDir != "" {
		if _, err := os.Stat(cfg.File.StorageDir); err == nil {
			r.Static("/uploads", cfg.File.StorageDir)
		}
	}
}
