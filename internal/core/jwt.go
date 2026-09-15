package core

import (
	"fmt"
	"strconv"
	"time"

	"gin-apeadmin/internal/config"
	"gin-apeadmin/internal/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims JWT Claims
type Claims struct {
	jwt.RegisteredClaims
	Type         string `json:"type"` // "access" | "refresh"
	Username      string `json:"username"`
	TokenVersion  int    `json:"tv"`
}

var jwtCfg *config.JWTConfig

// SetJWTConfig 初始化时注入配置
func SetJWTConfig(cfg *config.JWTConfig) {
	jwtCfg = cfg
}

func getJWTConfig() *config.JWTConfig {
	if jwtCfg == nil {
		panic("JWT config not initialized; call SetJWTConfig first")
	}
	return jwtCfg
}

// GenerateAccessToken 生成 access token
func GenerateAccessToken(user *model.SysUser) (string, error) {
	cfg := getJWTConfig()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Subject:   strconv.Itoa(int(user.ID)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.ExpireMinutes) * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Type:         "access",
		Username:     user.Username,
		TokenVersion: user.TokenVersion,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// GenerateRefreshToken 生成 refresh token
func GenerateRefreshToken(userID uint) (string, error) {
	cfg := getJWTConfig()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(int(userID)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(cfg.RefreshExpireDays) * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Type: "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.Secret))
}

// ParseToken 解析并验证 token
func ParseToken(tokenString string) (*Claims, error) {
	cfg := getJWTConfig()
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// 算法白名单：防 alg 替换攻击
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(cfg.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, err
	}
	return token.Claims.(*Claims), nil
}

// GetUserIDFromClaims 从 Claims 提取 UserID
func GetUserIDFromClaims(claims *Claims) (uint, error) {
	uid, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return 0, fmt.Errorf("invalid subject: %w", err)
	}
	return uint(uid), nil
}
