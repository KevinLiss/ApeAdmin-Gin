package api

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/pkg/response"
	"apeadmin-gin/internal/schema"
	"apeadmin-gin/internal/service"
)

// translateServiceErr 将 service 层返回的 key 化错误按请求语言翻译；非 key 化错误原样返回
func translateServiceErr(c *gin.Context, err error) string {
	var le *service.LoginError
	if ok := errors.As(err, &le); ok {
		return response.ErrorCtx(c, 0, le.Key, nil).Msg
	}
	return err.Error()
}

// T api 包内便捷翻译（handler 内拼 params 用）
func T(c *gin.Context, key string) string {
	return response.ErrorCtx(c, 0, key, nil).Msg
}

// AuthHandler 认证相关 Handler
type AuthHandler struct{}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req schema.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	result, err := service.Login(req.Username, req.Password, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Body{Code: 401, Msg: translateServiceErr(c, err)})
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// GetUserInfo 获取当前用户信息
func (h *AuthHandler) GetUserInfo(c *gin.Context) {
	userID := c.GetUint("user_id")
	info, err := service.GetUserInfo(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "auth.get_userinfo_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(info))
}

// GetProfile 获取个人中心完整信息
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	user, err := dal.GetUserWithRoles(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "common.not_found", map[string]string{"name": T(c, "user.label")}))
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
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	if err := service.UpdateProfile(userID, req); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.update_fail", nil))
		return
	}
	// 返回更新后的用户信息，前端需用返回值同步 profile 和 Pinia store
	user, err := dal.GetUserWithRoles(userID)
	if err != nil {
		c.JSON(http.StatusOK, response.SuccessCtx(c, "common.update_ok", nil))
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
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	if err := service.ChangePassword(userID, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, response.Body{Code: 400, Msg: translateServiceErr(c, err)})
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "auth.pwd_change_ok", nil))
}

// Logout 登出
func (h *AuthHandler) Logout(c *gin.Context) {
	jti, _ := c.Get("jti")
	jtiStr := ""
	if v, ok := jti.(string); ok {
		jtiStr = v
	}
	userID := c.GetUint("user_id")
	if err := service.Logout(userID, jtiStr); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "auth.logout_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "auth.logout_ok", nil))
}

// RefreshToken 刷新 token
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	result, err := service.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.Body{Code: 401, Msg: translateServiceErr(c, err)})
		return
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// HealthCheck 健康检查
// pid 用于前端「重启后端」后轮询比对新旧进程，判断服务是否已恢复
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, response.Success(gin.H{
		"status": "ok",
		"time":   time.Now().Format(time.RFC3339),
		"pid":    os.Getpid(),
	}))
}

// GetPublicSettings 获取公开设置
func GetPublicSettings(c *gin.Context) {
	settings, err := dal.ListPublicSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "setting.get_fail", nil))
		return
	}
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	c.JSON(http.StatusOK, response.Success(result))
}
