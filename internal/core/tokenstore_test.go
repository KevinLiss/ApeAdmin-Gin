package core

import (
	"testing"
	"time"
)

func TestMemoryTokenStoreRevokeAndCheck(t *testing.T) {
	s := NewMemoryTokenStore()
	ctx := t.Context()

	if revoked, _ := s.IsRevoked(ctx, "jti-1"); revoked {
		t.Fatal("unknown jti must not be revoked")
	}
	if err := s.Revoke(ctx, "jti-1", time.Hour); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	revoked, err := s.IsRevoked(ctx, "jti-1")
	if err != nil || !revoked {
		t.Fatalf("revoked=%v err=%v, want true", revoked, err)
	}
}

func TestMemoryTokenStoreExpiry(t *testing.T) {
	s := NewMemoryTokenStore()
	ctx := t.Context()

	if err := s.Revoke(ctx, "jti-short", 10*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(30 * time.Millisecond)
	revoked, _ := s.IsRevoked(ctx, "jti-short")
	if revoked {
		t.Fatal("expired entry must not be reported revoked")
	}
}

func TestMemoryTokenStoreCleanup(t *testing.T) {
	s := NewMemoryTokenStore()
	ctx := t.Context()
	_ = s.Revoke(ctx, "gone", time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	s.CleanupExpired()
	s.mu.RLock()
	n := len(s.revoked)
	s.mu.RUnlock()
	if n != 0 {
		t.Fatalf("cleanup left %d entries", n)
	}
}

func TestMemoryTokenStoreDistinctJTI(t *testing.T) {
	s := NewMemoryTokenStore()
	ctx := t.Context()
	_ = s.Revoke(ctx, "a", time.Hour)
	if revoked, _ := s.IsRevoked(ctx, "b"); revoked {
		t.Fatal("unrelated jti must stay valid")
	}
}
