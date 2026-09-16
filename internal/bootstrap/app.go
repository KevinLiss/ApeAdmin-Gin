package bootstrap

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"apeadmin-gin/internal/api"
	"apeadmin-gin/internal/config"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/mcp"
	"apeadmin-gin/internal/middleware"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/plugin"

	// L1 内置插件（blank import，触发 init() 自注册）
	_ "apeadmin-gin/internal/plugin/builtin/hello"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// allModels 所有需要迁移的模型
var allModels = []interface{}{
	&model.SysUser{},
	&model.SysRole{},
	&model.SysMenu{},
	&model.SysDept{},
	&model.SysPlugin{},
	&model.SysMcpTool{},
	&model.SysMcpAuditLog{},
	&model.SysLog{},
	&model.SysSetting{},
	&model.SysFile{},
	&model.SysFileFolder{},
	&model.SysAiProvider{},
	&model.SysChatSession{},
	&model.SysChatMessage{},
}

// Run 应用启动编排
func Run(configPath string) error {
	// 1. 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// 2. 初始化日志
	logger := core.InitLogger(cfg.Log)
	defer logger.Sync()

	// 3. 初始化数据库
	db, err := core.InitDB(&cfg.Database, allModels)
	if err != nil {
		return err
	}

	// 4. 种子数据
	core.SeedData(db, cfg.SuperAdmin)

	// 5. 注入容器
	core.SetConfig(cfg)
	core.SetDB(db)
	core.SetLogger(logger)

	// 6. TokenStore
	tokenStore := core.NewMemoryTokenStore()
	core.SetTokenStore(tokenStore)

	// 7. JWT 配置
	core.SetJWTConfig(&cfg.JWT)

	// 8. DAL 初始化
	dal.Init()

	// 9. 操作日志队列
	auditQueue := core.NewAuditQueue(db, cfg.Log.OpLogQueueSize)
	auditQueue.Start()
	core.SetAuditQueue(auditQueue)

	// 10. MCP 管理器
	mcpMgr := mcp.NewManager(db)
	mcpMgr.SetConfig(cfg)
	core.SetMCPManager(mcpMgr)
	mcp.RegisterBuiltin(mcpMgr)

	// 11. 插件管理器
	pluginMgr := plugin.NewManager(db, cfg)
	pluginMgr.SetMCPManager(mcpMgr)
	pluginMgr.Discover()

	// 12. 创建 Gin 引擎
	if !cfg.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.CORS(cfg.CORS.Origins))
	r.Use(middleware.Logger())
	r.Use(middleware.OperationLog())

	// 13. 注册路由
	api.RegisterRoutes(r, cfg, func(public, authed *gin.RouterGroup) {
		pluginMgr.RegisterAll(public, authed)
	})

	// 14. 启动事件
	pluginMgr.EmitEvent(plugin.EventAppStartup, nil)

	// 15. 启动 HTTP（优雅关闭）
	return runServer(r, cfg.App.Port, &shutdownCtx{
		pluginMgr:  pluginMgr,
		db:         db,
		auditQueue: auditQueue,
	})
}

// shutdownCtx 优雅关闭依赖
type shutdownCtx struct {
	pluginMgr  *plugin.Manager
	db         *gorm.DB
	auditQueue *core.AuditQueue
}

func runServer(r *gin.Engine, port int, deps *shutdownCtx) error {
	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		log.Printf("服务器启动: http://localhost:%d", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	core.SetShutdownCh(quit)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("正在关闭服务器...")

	// 阶段 1：停止接收新请求（5s）
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	srv.Shutdown(ctx1)

	// 阶段 2：插件卸载 + 事件（5s）
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	if deps.pluginMgr != nil {
		deps.pluginMgr.UninstallAll()
		deps.pluginMgr.EmitEvent(plugin.EventAppShutdown, nil)
	}

	// 阶段 3：日志 drain + DB 断连（5s）
	if deps.auditQueue != nil {
		deps.auditQueue.Shutdown(ctx2)
	}
	if deps.db != nil {
		if sqlDB, err := deps.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}

	log.Println("服务器已关闭")
	return nil
}
