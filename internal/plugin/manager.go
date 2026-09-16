package plugin

import (
	"context"
	"log"
	"sync"

	"gin-apeadmin/internal/config"
	"gin-apeadmin/internal/mcp"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Manager 插件管理器
type Manager struct {
	db        *gorm.DB
	cfg       *config.Config
	mcpMgr    *mcp.Manager
	plugins   map[string]Plugin
	pluginsMu sync.RWMutex
	eventBus  *EventBus
}

// NewManager 创建插件管理器
func NewManager(db *gorm.DB, cfg *config.Config) *Manager {
	return &Manager{
		db:       db,
		cfg:      cfg,
		plugins:  make(map[string]Plugin),
		eventBus: NewEventBus(),
	}
}

// SetMCPManager 注入 MCP 管理器（bootstrap 在注册路由前调用）
func (m *Manager) SetMCPManager(mcpMgr *mcp.Manager) {
	m.mcpMgr = mcpMgr
}

// EventBus 返回事件总线
func (m *Manager) EventBus() *EventBus {
	return m.eventBus
}

// Discover 发现并加载所有内置（L1）插件
// 流程：遍历全局注册表 → OnLoad → 注册到管理器 → 返回成功加载的插件列表
// OnLoad 失败的插件会被跳过（不阻止其他插件加载），但仍然保留在 plugins map 中
func (m *Manager) Discover() error {
	for _, p := range GetRegistered() {
		m.pluginsMu.Lock()
		m.plugins[p.Name()] = p
		m.pluginsMu.Unlock()
		if err := p.OnLoad(); err != nil {
			log.Printf("[plugin] OnLoad 失败 %s: %v（跳过）", p.Name(), err)
			continue
		}
		log.Printf("[plugin] 已加载: %s v%s", p.Name(), p.Version())
	}
	return nil
}

// RegisterAll 为所有已加载的插件创建 PluginRouter 并调用 Register
// 必须在路由注册之后调用（需要 public/authed RouterGroup）
func (m *Manager) RegisterAll(public, authed *gin.RouterGroup) error {
	m.pluginsMu.RLock()
	defer m.pluginsMu.RUnlock()

	for _, p := range m.plugins {
		router := &PluginRouter{
			Public: public,
			Authed: authed,
			MCP:    m.mcpMgr, // 可能为 nil（未注入 MCP 管理器时）
			DB:     m.db,
		}
		if err := p.Register(router); err != nil {
			log.Printf("[plugin] Register 失败 %s: %v（跳过）", p.Name(), err)
			continue
		}
		log.Printf("[plugin] 已注册路由: %s", p.Name())
	}
	return nil
}

// GetPlugin 获取插件实例
func (m *Manager) GetPlugin(name string) (Plugin, bool) {
	m.pluginsMu.RLock()
	defer m.pluginsMu.RUnlock()
	p, ok := m.plugins[name]
	return p, ok
}

// AllPlugins 返回所有已加载插件
func (m *Manager) AllPlugins() []Plugin {
	m.pluginsMu.RLock()
	defer m.pluginsMu.RUnlock()
	result := make([]Plugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		result = append(result, p)
	}
	return result
}

// UninstallAll 卸载所有插件
func (m *Manager) UninstallAll() {
	m.pluginsMu.RLock()
	defer m.pluginsMu.RUnlock()
	for _, p := range m.plugins {
		_ = p.Unregister()
		_ = p.Uninstall()
		p.OnUnload()
	}
}

// EmitEvent 发送事件
func (m *Manager) EmitEvent(name string, payload interface{}) {
	m.eventBus.Emit(context.Background(), name, payload)
}
