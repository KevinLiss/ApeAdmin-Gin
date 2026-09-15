package core

import (
	"fmt"
	"os"
	"sync"
	"syscall"

	"gin-apeadmin/internal/config"
	"gin-apeadmin/internal/mcp"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Container 全局依赖容器（取代裸全局变量）
// 运行期只读，Set* 仅限 bootstrap 初始化与测试
type Container struct {
	mu         sync.RWMutex
	db         *gorm.DB
	cfg        *config.Config
	logger      *zap.Logger
	tokenStore  TokenStore
	mcpManager  *mcp.Manager
	auditQueue  *AuditQueue
}

var global = &Container{}

func (c *Container) getDB() *gorm.DB {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db
}

func (c *Container) getCfg() *config.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg
}

func (c *Container) getLogger() *zap.Logger {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.logger
}

func (c *Container) getTokenStore() TokenStore {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tokenStore
}

func (c *Container) getMCPManager() *mcp.Manager {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.mcpManager
}

func (c *Container) getAuditQueue() *AuditQueue {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.auditQueue
}

func (c *Container) setDB(db *gorm.DB) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.db = db
}

func (c *Container) setCfg(cfg *config.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = cfg
}

func (c *Container) setLogger(l *zap.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = l
}

func (c *Container) setTokenStore(ts TokenStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokenStore = ts
}

func (c *Container) setMCPManager(m *mcp.Manager) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mcpManager = m
}

func (c *Container) setAuditQueue(q *AuditQueue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auditQueue = q
}

func (c *Container) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.db = nil
	c.cfg = nil
	c.logger = nil
	c.tokenStore = nil
	c.mcpManager = nil
	c.auditQueue = nil
}

// ─── 公共只读访问（业务代码使用）───

func GetDB() *gorm.DB            { return global.getDB() }
func GetConfig() *config.Config  { return global.getCfg() }
func GetLogger() *zap.Logger     { return global.getLogger() }
func GetTokenStore() TokenStore  { return global.getTokenStore() }
func GetMCPManager() *mcp.Manager { return global.getMCPManager() }
func GetAuditQueue() *AuditQueue { return global.getAuditQueue() }

// ─── Set（仅限 bootstrap 初始化与测试）───

func SetDB(db *gorm.DB)                { global.setDB(db) }
func SetConfig(cfg *config.Config)     { global.setCfg(cfg) }
func SetLogger(l *zap.Logger)          { global.setLogger(l) }
func SetTokenStore(ts TokenStore)      { global.setTokenStore(ts) }
func SetMCPManager(m *mcp.Manager)      { global.setMCPManager(m) }
func SetAuditQueue(q *AuditQueue)      { global.setAuditQueue(q) }

// ResetTest 测试专用：清空容器
func ResetTest() { global.reset() }

// MustGetDB 获取 DB，若未初始化则 panic
func MustGetDB() *gorm.DB {
	db := GetDB()
	if db == nil {
		panic(fmt.Sprintf("Container DB not initialized; call SetDB first"))
	}
	return db
}

// ─── 优雅关闭请求（由 Restart API 等触发）───

var shutdownCh chan os.Signal

// SetShutdownCh 注册优雅关闭通道（bootstrap 启动时调用）
func SetShutdownCh(ch chan os.Signal) { shutdownCh = ch }

// RequestShutdown 请求优雅关闭（非阻塞）
func RequestShutdown() {
	if shutdownCh != nil {
		select {
		case shutdownCh <- syscall.SIGTERM:
		default:
		}
	}
}
