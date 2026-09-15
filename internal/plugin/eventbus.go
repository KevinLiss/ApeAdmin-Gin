package plugin

import (
	"context"
	"sync"
)

// EventBus 事件总线
type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]EventHandler
}

// NewEventBus 创建事件总线
func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[string][]EventHandler),
	}
}

// Subscribe 订阅事件
func (b *EventBus) Subscribe(event string, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners[event] = append(b.listeners[event], handler)
}

// Emit 发送事件
func (b *EventBus) Emit(ctx context.Context, event string, payload interface{}) {
	b.mu.RLock()
	handlers := make([]EventHandler, len(b.listeners[event]))
	copy(handlers, b.listeners[event])
	b.mu.RUnlock()

	for _, h := range handlers {
		go h(ctx, payload)
	}
}
