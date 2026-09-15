package plugin

import "sync"

// Registry 全局插件注册表（init() 自注册）
type Registry struct {
	mu      sync.RWMutex
	plugins []Plugin
}

var globalRegistry = &Registry{}

// Register 由插件在 init() 中调用
func Register(p Plugin) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.plugins = append(globalRegistry.plugins, p)
}

// GetRegistered 返回所有已注册插件
func GetRegistered() []Plugin {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	result := make([]Plugin, len(globalRegistry.plugins))
	copy(result, globalRegistry.plugins)
	return result
}
