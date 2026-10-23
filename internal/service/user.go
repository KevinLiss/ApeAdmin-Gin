package service

import (
	"errors"
	"time"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/pkg/utils"
	"apeadmin-gin/internal/schema"
	"gorm.io/gorm"
)

// LoginError 业务错误（携带 i18n key，handler 层按请求语言翻译）
type LoginError struct {
	Key string
}

func (e *LoginError) Error() string {
	return core.T(core.DefaultLocale(), e.Key, nil)
}

// Login 用户登录
func Login(username, password, ip string) (*schema.LoginResponse, error) {
	user, err := dal.GetUserByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &LoginError{Key: "auth.wrong_credentials"}
		}
		return nil, err
	}
	if user.Status != 1 {
		return nil, &LoginError{Key: "auth.account_disabled"}
	}
	if !utils.CheckPassword(password, user.Password) {
		return nil, &LoginError{Key: "auth.wrong_credentials"}
	}

	// 更新最后登录信息
	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = ip
	if err := dal.UpdateUser(user); err != nil {
		return nil, err
	}

	accessToken, err := core.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	refreshToken, err := core.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}

	return &schema.LoginResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		RefreshToken: refreshToken,
	}, nil
}

// Logout 用户登出（拉黑当前 JTI + 移除在线会话记录）
func Logout(userID uint, jti string) error {
	core.RemoveOnline(userID, jti)
	cfg := core.GetConfig()
	tokenStore := core.GetTokenStore()
	if tokenStore == nil {
		return nil
	}
	// TTL = access token 剩余有效期
	ttl := time.Duration(cfg.JWT.ExpireMinutes) * time.Minute
	return tokenStore.Revoke(nil, jti, ttl)
}

// KickUser 强制下线（TokenVersion +1：该用户全部 access/refresh token 立即失效）
func KickUser(userID uint) error {
	return dal.IncrementTokenVersion(userID)
}

// ChangePassword 修改密码
func ChangePassword(userID uint, oldPassword, newPassword string) error {
	user, err := dal.GetUserByID(userID)
	if err != nil {
		return err
	}
	if !utils.CheckPassword(oldPassword, user.Password) {
		return &LoginError{Key: "auth.old_pwd_error"}
	}
	hash, err := utils.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := dal.UpdateUserPassword(userID, hash); err != nil {
		return err
	}
	// TokenVersion +1，全量失效旧 token
	return dal.IncrementTokenVersion(userID)
}

// UpdateProfile 更新个人资料
func UpdateProfile(userID uint, req schema.UpdateProfileRequest) error {
	user, err := dal.GetUserByID(userID)
	if err != nil {
		return err
	}
	if req.Nickname != "" {
		user.Nickname = req.Nickname
	}
	if req.Avatar != "" {
		user.Avatar = &req.Avatar
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	return dal.UpdateUser(user)
}

// RefreshToken 刷新 token
// 安全校验：tv 一致（改密/禁用后旧 refresh 立即失效）+ 旧 refresh jti 拉黑（轮换后不可重放）
func RefreshToken(refreshTokenStr string) (*schema.LoginResponse, error) {
	claims, err := core.ParseToken(refreshTokenStr)
	if err != nil || claims.Type != "refresh" {
		return nil, &LoginError{Key: "auth.refresh_invalid"}
	}
	// 黑名单校验：被轮换/登出拉黑的 refresh 不可重放
	cfg := core.GetConfig()
	if cfg.JWT.BlacklistEnabled {
		if tokenStore := core.GetTokenStore(); tokenStore != nil {
			if revoked, _ := tokenStore.IsRevoked(nil, claims.ID); revoked {
				return nil, &LoginError{Key: "auth.refresh_invalid"}
			}
		}
	}
	userID, err := core.GetUserIDFromClaims(claims)
	if err != nil {
		return nil, err
	}
	user, err := dal.GetUserByID(userID)
	if err != nil || user.Status != 1 {
		return nil, &LoginError{Key: "auth.user_disabled"}
	}
	// TokenVersion 校验：改密/重置密码/禁用后 tv 递增，旧 refresh 全部失效
	if user.TokenVersion != claims.TokenVersion {
		return nil, &LoginError{Key: "auth.refresh_invalid"}
	}
	accessToken, err := core.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	newRefresh, err := core.GenerateRefreshToken(user)
	if err != nil {
		return nil, err
	}
	// 轮换撤销：旧 refresh 拉黑至其自身过期（防重放）
	if cfg.JWT.BlacklistEnabled {
		if tokenStore := core.GetTokenStore(); tokenStore != nil {
			if exp := claims.ExpiresAt; exp != nil {
				ttl := time.Until(exp.Time)
				if ttl > 0 {
					_ = tokenStore.Revoke(nil, claims.ID, ttl)
				}
			}
		}
	}
	return &schema.LoginResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		RefreshToken: newRefresh,
	}, nil
}
