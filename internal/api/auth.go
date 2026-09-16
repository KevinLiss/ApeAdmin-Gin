package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/pkg/response"
	"apeadmin-gin/internal/schema"
	"apeadmin-gin/internal/service"
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

// GetProfile 获取个人中心完整信息
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	user, err := dal.GetUserWithRoles(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "用户不存在"))
		return
	}
	roles := make([]map[string]interface{}, 0, len(user.Roles))
	for _, r := range user.Roles {
		roles = append(roles, map[string]interface{}{"id": r.ID, "name": r.Name, "code": r.Code})
	}
	var dept map[string]interface{}
	if user.Dept != nil {
		dept = map[string]interface{}{"id": user.Dept.ID, "name": user.Dept.Name}
	}
	var lastLogin string
	if user.LastLoginAt != nil {
		lastLogin = user.LastLoginAt.Format(time.RFC3339)
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"id":            user.ID,
		"username":      user.Username,
		"nickname":      user.Nickname,
		"email":         user.Email,
		"phone":         user.Phone,
		"avatar":        user.Avatar,
		"dept_id":       user.DeptID,
		"dept":          dept,
		"roles":         roles,
		"status":        user.Status,
		"last_login_at": lastLogin,
		"last_login_ip": user.LastLoginIP,
		"created_at":    user.CreatedAt.Format(time.RFC3339),
	}))
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
	// 返回更新后的用户信息，前端需用返回值同步 profile 和 Pinia store
	user, err := dal.GetUserWithRoles(userID)
	if err != nil {
		c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
		return
	}
	roles := make([]map[string]interface{}, 0, len(user.Roles))
	for _, r := range user.Roles {
		roles = append(roles, map[string]interface{}{"id": r.ID, "name": r.Name, "code": r.Code})
	}
	var dept map[string]interface{}
	if user.Dept != nil {
		dept = map[string]interface{}{"id": user.Dept.ID, "name": user.Dept.Name}
	}
	var lastLogin string
	if user.LastLoginAt != nil {
		lastLogin = user.LastLoginAt.Format(time.RFC3339)
	}
	avatar := ""
	if user.Avatar != nil {
		avatar = *user.Avatar
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{
		"id":            user.ID,
		"username":      user.Username,
		"nickname":      user.Nickname,
		"email":         user.Email,
		"phone":         user.Phone,
		"avatar":        avatar,
		"dept_id":       user.DeptID,
		"dept":          dept,
		"roles":         roles,
		"status":        user.Status,
		"last_login_at": lastLogin,
		"last_login_ip": user.LastLoginIP,
		"created_at":    user.CreatedAt.Format(time.RFC3339),
	}))
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
