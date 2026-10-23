package core

import (
	"testing"
	"time"

	"apeadmin-gin/internal/config"
	"apeadmin-gin/internal/model"
)

func testJWTSetup() {
	SetJWTConfig(&config.JWTConfig{
		Secret:            "unit-test-secret-key-32-bytes-ok!!",
		Algorithm:         "HS256",
		ExpireMinutes:     60,
		RefreshExpireDays: 7,
		BlacklistEnabled:  true,
	})
}

func TestAccessTokenRoundTrip(t *testing.T) {
	testJWTSetup()
	user := &model.SysUser{Username: "alice", TokenVersion: 3}
	user.ID = 42
	token, err := GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Type != "access" {
		t.Errorf("type = %s, want access", claims.Type)
	}
	uid, _ := GetUserIDFromClaims(claims)
	if uid != 42 {
		t.Errorf("uid = %d, want 42", uid)
	}
	if claims.Username != "alice" {
		t.Errorf("username = %s, want alice", claims.Username)
	}
	if claims.TokenVersion != 3 {
		t.Errorf("tv = %d, want 3", claims.TokenVersion)
	}
}

func TestRefreshTokenCarriesTokenVersion(t *testing.T) {
	testJWTSetup()
	user := &model.SysUser{Username: "bob", TokenVersion: 5}
	user.ID = 7
	token, err := GenerateRefreshToken(user)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.Type != "refresh" {
		t.Errorf("type = %s, want refresh", claims.Type)
	}
	// P0 修复点：refresh token 必须携带 tv，改密后旧 refresh 失效
	if claims.TokenVersion != 5 {
		t.Errorf("refresh tv = %d, want 5（改密后旧 refresh 应可被判定失效）", claims.TokenVersion)
	}
	if claims.ID == "" {
		t.Errorf("refresh jti 为空（轮换撤销依赖 jti）")
	}
}

func TestParseTokenRejectsAlgSwap(t *testing.T) {
	testJWTSetup()
	// 构造无签名 token（alg=none 攻击模拟）
	tampered := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.e30."
	if _, err := ParseToken(tampered); err == nil {
		t.Fatalf("alg=none token 不应被接受")
	}
}

func TestParseTokenWrongSecret(t *testing.T) {
	testJWTSetup()
	user := &model.SysUser{Username: "x"}
	token, _ := GenerateAccessToken(user)
	// 换 secret
	SetJWTConfig(&config.JWTConfig{Secret: "another-secret-another-secret-32b!", ExpireMinutes: 60, RefreshExpireDays: 7})
	if _, err := ParseToken(token); err == nil {
		t.Fatalf("错误 secret 签发的 token 不应通过校验")
	}
}

func TestTokenStoreRevokeAndExpire(t *testing.T) {
	s := NewMemoryTokenStore()
	_ = s.Revoke(nil, "jti-1", 50*time.Millisecond)
	if ok, _ := s.IsRevoked(nil, "jti-1"); !ok {
		t.Fatalf("撤销后应命中黑名单")
	}
	time.Sleep(60 * time.Millisecond)
	if ok, _ := s.IsRevoked(nil, "jti-1"); ok {
		t.Fatalf("过期后应自动移出黑名单")
	}
}

func TestOnlineTrackListKickSweep(t *testing.T) {
	TrackOnline(1, "alice", "jti-a", "1.1.1.1", "ua")
	TrackOnline(2, "bob", "jti-b", "2.2.2.2", "ua")
	// 同一用户多端
	TrackOnline(1, "alice", "jti-a2", "1.1.1.1", "ua")
	sessions := ListOnline()
	if len(sessions) != 3 {
		t.Fatalf("在线会话数 = %d, want 3", len(sessions))
	}
	RemoveOnline(1, "jti-a")
	sessions = ListOnline()
	if len(sessions) != 2 {
		t.Fatalf("移除后会话数 = %d, want 2", len(sessions))
	}
	// 最近活跃排序（jti-a2 后登记应在最前）
	if sessions[0].UserID != 1 {
		t.Fatalf("排序首位 UserID = %d, want 1（最近活跃优先）", sessions[0].UserID)
	}
}
