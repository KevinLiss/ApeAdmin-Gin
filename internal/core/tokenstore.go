package core

import (
	"context"
	"sync"
	"time"
)

// TokenStore 接口化：单实例内存实现，多实例 Redis 实现
type TokenStore interface {
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// MemoryTokenStore 内存黑名单实现（默认，单实例部署）
type MemoryTokenStore struct {
	mu     sync.RWMutex
	revoked map[string]time.Time // jti → expireAt
}

func NewMemoryTokenStore() *MemoryTokenStore {
	return &MemoryTokenStore{
		revoked: make(map[string]time.Time),
	}
}

func (s *MemoryTokenStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revoked[jti] = time.Now().Add(ttl)
	return nil
}

func (s *MemoryTokenStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	s.mu.RLock()
	expireAt, ok := s.revoked[jti]
	s.mu.RUnlock()
	if !ok {
		return false, nil
	}
	// 过期自动清理
	if time.Now().After(expireAt) {
		s.mu.Lock()
		delete(s.revoked, jti)
		s.mu.Unlock()
		return false, nil
	}
	return true, nil
}

// CleanupExpired 清理过期条目（可由后台定时任务调用）
func (s *MemoryTokenStore) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for jti, expireAt := range s.revoked {
		if now.After(expireAt) {
			delete(s.revoked, jti)
		}
	}
}
