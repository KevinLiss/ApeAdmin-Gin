package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/response"
	"apeadmin-gin/internal/pkg/utils"
	"apeadmin-gin/internal/service"

	"gorm.io/gorm"
)

type UserHandler struct{}

func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	// 数据权限：按当前用户角色的 data_scope 过滤（超管/全部角色不过滤）
	var scope func(*gorm.DB) *gorm.DB
	if userVal, ok := c.Get("user"); ok {
		if u, uok := userVal.(*model.SysUser); uok && !u.IsSuperAdmin {
			scope = dal.DataScopeScope(u, "id", "dept_id")
		}
	}

	users, total, err := dal.ListUsers(page, pageSize, keyword, scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessPage(users, total, page, pageSize))
}

func (h *UserHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	user, err := dal.GetUserWithRoles(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "common.not_found", map[string]string{"name": T(c, "user.label")}))
		return
	}
	c.JSON(http.StatusOK, response.Success(user))
}

func (h *UserHandler) Create(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Nickname string `json:"nickname"`
		Password string `json:"password" binding:"required,min=6"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		DeptID   *uint  `json:"dept_id"`
		RoleIDs  []uint `json:"role_ids"`
		Status   int    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "user.pwd_hash_fail", nil))
		return
	}

	user := model.SysUser{
		Username: req.Username,
		Nickname: req.Nickname,
		Password: hash,
		Email:    req.Email,
		Phone:    req.Phone,
		DeptID:   req.DeptID,
		Status:   req.Status,
	}
	if user.Status == 0 {
		user.Status = 1
	}
	if err := dal.CreateUser(&user); err != nil {
		c.JSON(http.StatusConflict, response.ErrorCtx(c, 409, "user.username_exists", nil))
		return
	}
	if len(req.RoleIDs) > 0 {
		dal.AssignUserRoles(user.ID, req.RoleIDs)
		service.InvalidateUserPermCache(user.ID) // 用户角色变更，失效其权限缓存
	}
	c.JSON(http.StatusOK, response.Success(user))
}

func (h *UserHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	user, err := dal.GetUserByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "common.not_found", map[string]string{"name": T(c, "user.label")}))
		return
	}

	var req struct {
		Nickname string `json:"nickname"`
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		DeptID   *uint  `json:"dept_id"`
		RoleIDs  []uint `json:"role_ids"`
		Status   *int   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}

	if req.Nickname != "" { user.Nickname = req.Nickname }
	if req.Email != "" { user.Email = req.Email }
	if req.Phone != "" { user.Phone = req.Phone }
	if req.DeptID != nil { user.DeptID = req.DeptID }
	if req.Status != nil { user.Status = *req.Status }

	if err := dal.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.update_fail", nil))
		return
	}
	if req.RoleIDs != nil {
		dal.AssignUserRoles(user.ID, req.RoleIDs)
		service.InvalidateUserPermCache(user.ID) // 用户角色变更，失效其权限缓存
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.update_ok", nil))
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	// 保护 1：不能删除自己（避免误操作导致当前会话立即失效）
	currentUserID := c.GetUint("user_id")
	if currentUserID == uint(id) {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "user.cannot_delete_self", nil))
		return
	}
	// 保护 2：不能删除最后一个超级管理员（否则系统只能改库恢复）
	target, err := dal.GetUserByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "common.not_found", map[string]string{"name": T(c, "user.label")}))
		return
	}
	if target.IsSuperAdmin {
		saCount, saErr := dal.CountSuperAdmins()
		if saErr == nil && saCount <= 1 {
			c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "user.last_super_admin", nil))
			return
		}
	}
	if err := dal.DeleteUser(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.delete_fail", nil))
		return
	}
	service.InvalidateUserPermCache(uint(id)) // 删除用户，失效其权限缓存
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.delete_ok", nil))
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "user.pwd_hash_fail", nil))
		return
	}
	if err := dal.UpdateUserPassword(uint(id), hash); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.reset_fail", nil))
		return
	}
	// TokenVersion +1，全量失效旧 token
	dal.IncrementTokenVersion(uint(id))
	c.JSON(http.StatusOK, response.SuccessCtx(c, "auth.pwd_reset_ok", nil))
}

// OnlineList 在线用户列表（15 分钟活跃窗口内的会话）
func (h *UserHandler) OnlineList(c *gin.Context) {
	sessions := core.ListOnline()
	c.JSON(http.StatusOK, response.Success(gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	}))
}

// Kick 强制下线（TokenVersion +1，该用户全部 token 立即失效）
func (h *UserHandler) Kick(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	// 不能踢自己（与删除保护同理）
	currentUserID := c.GetUint("user_id")
	if currentUserID == uint(id) {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "user.cannot_delete_self", nil))
		return
	}
	if _, err := dal.GetUserByID(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "common.not_found", map[string]string{"name": T(c, "user.label")}))
		return
	}
	if err := service.KickUser(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.operation_fail", nil))
		return
	}
	service.InvalidateUserPermCache(uint(id))
	c.JSON(http.StatusOK, response.SuccessCtx(c, "user.kicked", nil))
}
