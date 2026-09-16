package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/pkg/response"
)

type RoleHandler struct{}

func (h *RoleHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	roles, total, err := dal.ListRoles(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessPage(roles, total, page, pageSize))
}

func (h *RoleHandler) ListAll(c *gin.Context) {
	roles, err := dal.ListAllRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(roles))
}

func (h *RoleHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	role, err := dal.GetRoleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "角色不存在"))
		return
	}
	c.JSON(http.StatusOK, response.Success(role))
}

func (h *RoleHandler) Create(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Code      string `json:"code" binding:"required"`
		DataScope int    `json:"data_scope"`
		Sort      int    `json:"sort"`
		Remark    string `json:"remark"`
		MenuIDs   []uint `json:"menu_ids"`
		Status    int    `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	role := model.SysRole{
		Name:      req.Name,
		Code:      req.Code,
		DataScope: req.DataScope,
		Sort:      req.Sort,
		Remark:    req.Remark,
		Status:    req.Status,
	}
	if role.Status == 0 { role.Status = 1 }
	if role.DataScope == 0 { role.DataScope = 1 }
	if err := dal.CreateRole(&role); err != nil {
		c.JSON(http.StatusConflict, response.Error(409, "角色编码已存在"))
		return
	}
	if len(req.MenuIDs) > 0 {
		dal.AssignRoleMenus(role.ID, req.MenuIDs)
	}
	c.JSON(http.StatusOK, response.Success(role))
}

func (h *RoleHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	role, err := dal.GetRoleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "角色不存在"))
		return
	}
	var req struct {
		Name      string `json:"name"`
		DataScope int    `json:"data_scope"`
		Sort      int    `json:"sort"`
		Remark    string `json:"remark"`
		MenuIDs   []uint `json:"menu_ids"`
		Status    *int   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	if req.Name != "" { role.Name = req.Name }
	if req.DataScope != 0 { role.DataScope = req.DataScope }
	role.Sort = req.Sort
	role.Remark = req.Remark
	if req.Status != nil { role.Status = *req.Status }
	if err := dal.UpdateRole(role); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "更新失败"))
		return
	}
	if req.MenuIDs != nil {
		dal.AssignRoleMenus(role.ID, req.MenuIDs)
	}
	c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
}

func (h *RoleHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := dal.DeleteRole(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "删除失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("删除成功"))
}

// ─── 菜单 ───

type MenuHandler struct{}

func (h *MenuHandler) Tree(c *gin.Context) {
	menus, err := dal.GetMenuTree()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(menus))
}

func (h *MenuHandler) Create(c *gin.Context) {
	var menu model.SysMenu
	if err := c.ShouldBindJSON(&menu); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	if menu.Status == 0 { menu.Status = 1 }
	if menu.Visible == 0 { menu.Visible = 1 }
	if err := dal.CreateMenu(&menu); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "创建失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(menu))
}

func (h *MenuHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var m model.SysMenu
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	m.ID = uint(id)
	if err := dal.UpdateMenu(&m); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "更新失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
}

func (h *MenuHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := dal.DeleteMenu(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "删除失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("删除成功"))
}

// ─── 部门 ───

type DeptHandler struct{}

func (h *DeptHandler) Tree(c *gin.Context) {
	depts, err := dal.GetAllDepts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(depts))
}

func (h *DeptHandler) Create(c *gin.Context) {
	var dept model.SysDept
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	if dept.Status == 0 { dept.Status = 1 }
	if err := dal.CreateDept(&dept); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "创建失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(dept))
}

func (h *DeptHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var dept model.SysDept
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	dept.ID = uint(id)
	if err := dal.UpdateDept(&dept); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "更新失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
}

func (h *DeptHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := dal.DeleteDept(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "删除失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("删除成功"))
}

// ─── 日志 ───

type LogHandler struct{}

func (h *LogHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	logs, total, err := dal.ListLogs(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessPage(logs, total, page, pageSize))
}

func (h *LogHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	log, err := dal.GetLogByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "日志不存在"))
		return
	}
	c.JSON(http.StatusOK, response.Success(log))
}

func (h *LogHandler) Clear(c *gin.Context) {
	if err := dal.ClearLogs(); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "清空失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("清空成功"))
}

func (h *LogHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := dal.DeleteLog(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "删除失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("删除成功"))
}

// ─── 系统设置 ───

type SettingHandler struct{}

func (h *SettingHandler) List(c *gin.Context) {
	settings, err := dal.ListSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(settings))
}

func (h *SettingHandler) BatchUpdate(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	if err := dal.BatchUpdateSettings(req); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "更新失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
}

func (h *SettingHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	if err := dal.UpdateSetting(key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "更新失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("更新成功"))
}
