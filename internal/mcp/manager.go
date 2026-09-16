package mcp

import (
	"encoding/json"
	"strings"
	"sync"

	"apeadmin-gin/internal/config"
	"apeadmin-gin/internal/model"
	"gorm.io/gorm"
)

// Manager MCP 管理器
type Manager struct {
	db        *gorm.DB
	cfg       *config.Config
	mu        sync.RWMutex
	tools     map[string]*ToolEntry
	resources map[string]*ResourceEntry
	prompts   map[string]*PromptEntry
}

// Registrar 插件注册 MCP 工具的入口（等价于 *Manager）
type Registrar = Manager

// ToolEntry MCP 工具注册条目
type ToolEntry struct {
	Name                string
	Description         string
	InputSchema         map[string]interface{}
	PluginName          string
	Category            string
	RequiredPermissions []string
	Handler             ToolHandler
}

// ResourceEntry MCP 资源注册条目
type ResourceEntry struct {
	URI         string
	Name        string
	Description string
	MimeType    string
	ReadHandler func() (string, error)
}

// PromptEntry MCP 提示词注册条目
type PromptEntry struct {
	Name        string
	Description string
	Arguments   []string
	Template    string // 支持 {arg} 占位符
}

// ToolHandler MCP 工具处理函数
type ToolHandler func(args map[string]interface{}) (interface{}, error)

// NewManager 创建 MCP 管理器
func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db:        db,
		tools:     make(map[string]*ToolEntry),
		resources: make(map[string]*ResourceEntry),
		prompts:   make(map[string]*PromptEntry),
	}
}

// SetConfig 注入配置引用（bootstrap 调用）
func (m *Manager) SetConfig(cfg *config.Config) {
	m.cfg = cfg
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *config.Config {
	return m.cfg
}

// GetDB 获取数据库
func (m *Manager) GetDB() *gorm.DB {
	return m.db
}

// RegisterTool 注册 MCP 工具
func (m *Manager) RegisterTool(entry *ToolEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry.Category == "" {
		entry.Category = "system"
	}
	m.tools[entry.Name] = entry
}

// UnregisterPluginTools 注销某插件的所有工具
func (m *Manager) UnregisterPluginTools(pluginName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, t := range m.tools {
		if t.PluginName == pluginName {
			delete(m.tools, name)
		}
	}
}

// ListTools 列出所有工具（按名称排序）
func (m *Manager) ListTools() []*ToolEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ToolEntry, 0, len(m.tools))
	for _, t := range m.tools {
		result = append(result, t)
	}
	// 简单排序保证顺序稳定
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Name < result[i].Name {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// GetTool 按名称查找工具
func (m *Manager) GetTool(name string) (*ToolEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tools[name]
	return t, ok
}

// ListCategories 工具分类统计
func (m *Manager) ListCategories() []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	counter := make(map[string]int)
	for _, t := range m.tools {
		cat := t.Category
		if cat == "" {
			cat = "system"
		}
		counter[cat]++
	}
	result := make([]map[string]interface{}, 0, len(counter))
	for name, count := range counter {
		result = append(result, map[string]interface{}{"name": name, "count": count})
	}
	return result
}

// RegisterResource 注册资源
func (m *Manager) RegisterResource(entry *ResourceEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources[entry.URI] = entry
}

// ListResources 列出所有资源
func (m *Manager) ListResources() []*ResourceEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ResourceEntry, 0, len(m.resources))
	for _, r := range m.resources {
		result = append(result, r)
	}
	return result
}

// ReadResource 读取资源内容
func (m *Manager) ReadResource(uri string) (string, bool) {
	m.mu.RLock()
	r, ok := m.resources[uri]
	m.mu.RUnlock()
	if !ok {
		return "", false
	}
	content, err := r.ReadHandler()
	if err != nil {
		return "", false
	}
	return content, true
}

// RegisterPrompt 注册提示词
func (m *Manager) RegisterPrompt(entry *PromptEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.prompts[entry.Name] = entry
}

// ListPrompts 列出所有提示词
func (m *Manager) ListPrompts() []*PromptEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*PromptEntry, 0, len(m.prompts))
	for _, p := range m.prompts {
		result = append(result, p)
	}
	return result
}

// RenderPrompt 渲染提示词
func (m *Manager) RenderPrompt(name string, args map[string]string) (string, bool) {
	m.mu.RLock()
	p, ok := m.prompts[name]
	m.mu.RUnlock()
	if !ok {
		return "", false
	}
	rendered := p.Template
	for k, v := range args {
		rendered = strings.ReplaceAll(rendered, "{"+k+"}", v)
	}
	return rendered, true
}

// WriteAuditLog 写入 MCP 审计日志
func (m *Manager) WriteAuditLog(userID uint, username, actionType, targetName, arguments, result, status string, durationMs int64) {
	if m.db == nil {
		return
	}
	truncate := func(s string, n int) string {
		if len(s) > n {
			return s[:n]
		}
		return s
	}
	log := model.SysMcpAuditLog{
		UserID:     userID,
		Username:   username,
		ActionType: actionType,
		TargetName: targetName,
		Arguments:  truncate(arguments, 2000),
		Result:     truncate(result, 2000),
		Status:     status,
		DurationMs: durationMs,
	}
	m.db.Create(&log)
}

// MarshalSchema 序列化 input_schema（辅助函数）
func MarshalSchema(schema map[string]interface{}) string {
	if schema == nil {
		return ""
	}
	b, err := json.Marshal(schema)
	if err != nil {
		return ""
	}
	return string(b)
}
