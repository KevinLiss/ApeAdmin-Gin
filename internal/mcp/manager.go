package mcp

import (
	"sync"

	"gorm.io/gorm"
)

// Manager MCP 管理器（Phase 2 实现）
type Manager struct {
	db    *gorm.DB
	mu    sync.RWMutex
	tools map[string]*ToolEntry
}

// ToolEntry MCP 工具注册条目
type ToolEntry struct {
	Name        string
	Description string
	InputSchema map[string]interface{}
	PluginName  string
	Handler     ToolHandler
}

// ToolHandler MCP 工具处理函数
type ToolHandler func(args map[string]interface{}) (interface{}, error)

// NewManager 创建 MCP 管理器
func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db:    db,
		tools: make(map[string]*ToolEntry),
	}
}

// RegisterTool 注册 MCP 工具
func (m *Manager) RegisterTool(entry *ToolEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools[entry.Name] = entry
}

// UnregisterTool 注销 MCP 工具
func (m *Manager) UnregisterTool(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tools, name)
}

// ListTools 列出所有工具
func (m *Manager) ListTools() []*ToolEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ToolEntry, 0, len(m.tools))
	for _, t := range m.tools {
		result = append(result, t)
	}
	return result
}

// Registrar MCP 工具注册器（供插件使用）
type Registrar struct {
	manager    *Manager
	pluginName string
}

// NewRegistrar 创建注册器
func NewRegistrar(m *Manager, pluginName string) *Registrar {
	return &Registrar{manager: m, pluginName: pluginName}
}

// RegisterTool 注册工具
func (r *Registrar) RegisterTool(name, description string, schema map[string]interface{}, handler ToolHandler) {
	r.manager.RegisterTool(&ToolEntry{
		Name:        name,
		Description: description,
		InputSchema: schema,
		PluginName:  r.pluginName,
		Handler:     handler,
	})
}
