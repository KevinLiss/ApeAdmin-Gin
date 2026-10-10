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
	refreshToken, err := core.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &schema.LoginResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		RefreshToken: refreshToken,
	}, nil
}

// Logout 用户登出（拉黑当前 JTI）
func Logout(jti string) error {
	cfg := core.GetConfig()
	tokenStore := core.GetTokenStore()
	if tokenStore == nil {
		return nil
	}
	// TTL = access token 剩余有效期
	ttl := time.Duration(cfg.JWT.ExpireMinutes) * time.Minute
	return tokenStore.Revoke(nil, jti, ttl)
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
func RefreshToken(refreshTokenStr string) (*schema.LoginResponse, error) {
	claims, err := core.ParseToken(refreshTokenStr)
	if err != nil || claims.Type != "refresh" {
		return nil, &LoginError{Key: "auth.refresh_invalid"}
	}
	userID, err := core.GetUserIDFromClaims(claims)
	if err != nil {
		return nil, err
	}
	user, err := dal.GetUserByID(userID)
	if err != nil || user.Status != 1 {
		return nil, &LoginError{Key: "auth.user_disabled"}
	}
	accessToken, err := core.GenerateAccessToken(user)
	if err != nil {
		return nil, err
	}
	newRefresh, err := core.GenerateRefreshToken(userID)
	if err != nil {
		return nil, err
	}
	return &schema.LoginResponse{
		AccessToken:  accessToken,
		TokenType:    "bearer",
		RefreshToken: newRefresh,
	}, nil
}
