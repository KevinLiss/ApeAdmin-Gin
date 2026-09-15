package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gin-apeadmin/internal/dal"
	"gin-apeadmin/internal/pkg/response"
	"gin-apeadmin/internal/schema"
	"gin-apeadmin/internal/service"
)

// AuthHandler 认证相关 Handler
type AuthHandler struct{}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req schema.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	result, err := service.Login(req.Username, req.Password, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Error(401, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// GetUserInfo 获取当前用户信息
func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	userID := c.GetUint("user_id")
	info, err := service.GetUserInfo(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "获取用户信息失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(info))
}

// UpdateProfile 更新个人资料
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req schema.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	if err := service.UpdateProfile(userID, req); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "更新失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req schema.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	if err := service.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("密码修改成功"))
}

// Logout 登出
func (h *AuthHandler) Logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	jtiStr := ""
	if v, ok := jti.(string); ok {
		jtiStr = v
	}
	if err := service.Logout(jtiStr); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "登出失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("登出成功"))
}

// RefreshToken 刷新 token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	result, err := service.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Error(401, err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// HealthCheck 健康检查
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
	}))
}

// GetPublicSettings 获取公开设置
func GetPublicSettings(c *gin.Context) {
	settings, err := dal.ListPublicSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "获取设置失败"))
		return
	}
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	c.JSON(http.StatusOK, response.Success(result))
}
