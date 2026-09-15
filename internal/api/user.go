package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gin-apeadmin/internal/dal"
	"gin-apeadmin/internal/model"
	"gin-apeadmin/internal/pkg/response"
	"gin-apeadmin/internal/pkg/utils"
)

type UserHandler struct{}

func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	users, total, err := dal.ListUsers(page, pageSize, keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessPage(users, total, page, pageSize))
}

func (h *UserHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	user, err := dal.GetUserWithRoles(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "用户不存在"))
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
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}

	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "密码加密失败"))
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
		c.JSON(http.StatusConflict, response.Error(409, "用户名已存在"))
		return
	}
	if len(req.RoleIDs) > 0 {
		dal.AssignUserRoles(user.ID, req.RoleIDs)
	}
	c.JSON(http.StatusOK, response.Success(user))
}

func (h *UserHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	user, err := dal.GetUserByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "用户不存在"))
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
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}

	if req.Nickname != "" { user.Nickname = req.Nickname }
	if req.Email != "" { user.Email = req.Email }
	if req.Phone != "" { user.Phone = req.Phone }
	if req.DeptID != nil { user.DeptID = req.DeptID }
	if req.Status != nil { user.Status = *req.Status }

	if err := dal.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "更新失败"))
		return
	}
	if req.RoleIDs != nil {
		dal.AssignUserRoles(user.ID, req.RoleIDs)
	}
	c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := dal.DeleteUser(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "删除失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("删除成功"))
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Password string `json:"password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	hash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "密码加密失败"))
		return
	}
	if err := dal.UpdateUserPassword(uint(id), hash); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "重置失败"))
		return
	}
	// TokenVersion +1，全量失效旧 token
	dal.IncrementTokenVersion(uint(id))
	c.JSON(http.StatusOK, response.SuccessMsg("密码重置成功"))
}
