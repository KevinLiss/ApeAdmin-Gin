package plugin

import (
	"context"
	"sync"

	"gin-apeadmin/internal/config"
	"gorm.io/gorm"
)

// Manager 插件管理器
type Manager struct {
	db       *gorm.DB
	cfg      *config.Config
	plugins  map[string]Plugin
	pluginsMu sync.RWMutex
	eventBus *EventBus
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

// EventBus 返回事件总线
func (m *Manager) EventBus() *EventBus {
	return m.eventBus
}

// Discover 发现并加载所有内置插件
func (m *Manager) Discover() error {
	for _, p := range GetRegistered() {
		m.pluginsMu.Lock()
		m.plugins[p.Name()] = p
		m.pluginsMu.Unlock()
		if err := p.OnLoad(); err != nil {
			continue
		}
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
