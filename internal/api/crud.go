package api

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/pkg/response"
	"apeadmin-gin/internal/service"
)

type RoleHandler struct{}

func (h *RoleHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	roles, total, err := dal.ListRoles(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessPage(roles, total, page, pageSize))
}

func (h *RoleHandler) ListAll(c *gin.Context) {
	roles, err := dal.ListAllRoles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(roles))
}

func (h *RoleHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	role, err := dal.GetRoleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "role.not_found", nil))
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
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
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
		c.JSON(http.StatusConflict, response.ErrorCtx(c, 409, "role.code_exists", nil))
		return
	}
	if len(req.MenuIDs) > 0 {
		dal.AssignRoleMenus(role.ID, req.MenuIDs)
	}
	service.InvalidateAllPermCache() // 角色菜单变更，全量失效权限缓存
	c.JSON(http.StatusOK, response.Success(role))
}

func (h *RoleHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	role, err := dal.GetRoleByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "role.not_found", nil))
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
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	if req.Name != "" { role.Name = req.Name }
	if req.DataScope != 0 { role.DataScope = req.DataScope }
	role.Sort = req.Sort
	role.Remark = req.Remark
	if req.Status != nil { role.Status = *req.Status }
	if err := dal.UpdateRole(role); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.update_fail", nil))
		return
	}
	if req.MenuIDs != nil {
		dal.AssignRoleMenus(role.ID, req.MenuIDs)
	}
	service.InvalidateAllPermCache() // 角色菜单变更，全量失效权限缓存
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.update_ok", nil))
}

func (h *RoleHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := dal.DeleteRole(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.delete_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.delete_ok", nil))
}

// ─── 菜单 ───

type MenuHandler struct{}

func (h *MenuHandler) Tree(c *gin.Context) {
	menus, err := dal.GetMenuTree()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(menus))
}

func (h *MenuHandler) Create(c *gin.Context) {
	var menu model.SysMenu
	if err := c.ShouldBindJSON(&menu); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	if menu.Status == 0 { menu.Status = 1 }
	if menu.Visible == 0 { menu.Visible = 1 }
	if err := dal.CreateMenu(&menu); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.create_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(menu))
}

func (h *MenuHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var m model.SysMenu
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	m.ID = uint(id)
	if err := dal.UpdateMenu(&m); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.update_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.update_ok", nil))
}

func (h *MenuHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	// 有子菜单时禁止删除（避免悬空菜单节点）
	if childCount, err := dal.CountChildMenus(uint(id)); err == nil && childCount > 0 {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "menu.has_children", nil))
		return
	}
	if err := dal.DeleteMenu(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.delete_fail", nil))
		return
	}
	service.InvalidateAllPermCache() // 菜单删除影响权限码，全量失效缓存
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.delete_ok", nil))
}

// ─── 部门 ───

type DeptHandler struct{}

func (h *DeptHandler) Tree(c *gin.Context) {
	depts, err := dal.GetAllDepts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(depts))
}

func (h *DeptHandler) Create(c *gin.Context) {
	var dept model.SysDept
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	if dept.Name == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	// 上级部门存在性校验（parent_id=0 表示顶级）
	if dept.ParentID > 0 {
		if _, err := dal.GetDeptByID(dept.ParentID); err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "dept.parent_not_found", nil))
			return
		}
	}
	// 同级同名查重
	if exists, err := dal.ExistsDeptName(dept.ParentID, dept.Name, 0); err == nil && exists {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "dept.name_exists", nil))
		return
	}
	if dept.Status == 0 { dept.Status = 1 }
	if err := dal.CreateDept(&dept); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.create_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(dept))
}

func (h *DeptHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var dept model.SysDept
	if err := c.ShouldBindJSON(&dept); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	dept.ID = uint(id)
	if dept.Name == "" {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	// 上级部门校验：存在性 + 不能选自身（环）+ 不能选自己的子孙（环）
	if dept.ParentID > 0 {
		if dept.ParentID == dept.ID {
			c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "dept.parent_invalid", nil))
			return
		}
		if _, err := dal.GetDeptByID(dept.ParentID); err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "dept.parent_not_found", nil))
			return
		}
		if dal.IsDeptDescendant(dept.ID, dept.ParentID) {
			c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "dept.parent_invalid", nil))
			return
		}
	}
	// 同级同名查重（排除自身）
	if exists, err := dal.ExistsDeptName(dept.ParentID, dept.Name, dept.ID); err == nil && exists {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "dept.name_exists", nil))
		return
	}
	if err := dal.UpdateDept(&dept); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.update_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.update_ok", nil))
}

func (h *DeptHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	// 存在子部门时禁止删除（避免孤儿部门）
	if childCount, err := dal.CountChildDepts(uint(id)); err == nil && childCount > 0 {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "dept.has_children", nil))
		return
	}
	// 部门下还有用户时禁止删除（避免用户外键悬挂）
	if userCount, err := dal.CountUsersInDept(uint(id)); err == nil && userCount > 0 {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "dept.has_users", nil))
		return
	}
	if err := dal.DeleteDept(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.delete_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.delete_ok", nil))
}

// ─── 日志 ───

type LogHandler struct{}

func (h *LogHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	logs, total, err := dal.ListLogs(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessPage(logs, total, page, pageSize))
}

func (h *LogHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	log, err := dal.GetLogByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.ErrorCtx(c, 404, "log.not_found", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(log))
}

func (h *LogHandler) Clear(c *gin.Context) {
	if err := dal.ClearLogs(); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.clear_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.clear_ok", nil))
}

func (h *LogHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if err := dal.DeleteLog(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.delete_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.delete_ok", nil))
}

// ─── 系统设置 ───

type SettingHandler struct{}

func (h *SettingHandler) List(c *gin.Context) {
	settings, err := dal.ListSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.query_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.Success(settings))
}

func (h *SettingHandler) BatchUpdate(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	if err := dal.BatchUpdateSettings(req); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.update_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.update_ok", nil))
}

func (h *SettingHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.param_error", nil))
		return
	}
	if err := dal.UpdateSetting(key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.update_fail", nil))
		return
	}
	c.JSON(http.StatusOK, response.SuccessCtx(c, "common.update_ok", nil))
}

// BrandImage 品牌图片上传（Logo / 登录页背景），返回可访问的 URL
func (h *SettingHandler) BrandImage(c *gin.Context) {
	cfg := core.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "common.config_missing", nil))
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "common.file_missing", nil))
		return
	}

	// 扩展名校验（仅图片）
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true, ".ico": true}
	if !allowed[ext] {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "file.image_only", nil))
		return
	}

	// 大小校验（10MB）
	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "file.image_too_large", nil))
		return
	}

	// SVG 内容安全检查：拒绝含脚本/事件属性的 SVG（存储型 XSS 防护）
	if ext == ".svg" {
		if err := checkSVGSafety(file); err != nil {
			c.JSON(http.StatusBadRequest, response.ErrorCtx(c, 400, "file.svg_unsafe", nil))
			return
		}
	}

	// 保存到 uploads/brand/ 目录
	dir := filepath.Join(cfg.File.StorageDir, "brand")
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "file.mkdir_fail", nil))
		return
	}
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(dir, filename)
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorCtx(c, 500, "file.write_fail", nil))
		return
	}

	// 返回可访问 URL（通过静态托管 /uploads 前缀映射到 storage_dir）
	url := fmt.Sprintf("/uploads/brand/%s", filename)
	c.JSON(http.StatusOK, response.Success(gin.H{"url": url}))
}

// checkSVGSafety 检查 SVG 内容是否含可执行脚本（script 标签 / on* 事件属性 / javascript: 协议）
func checkSVGSafety(fileHeader *multipart.FileHeader) error {
	if fileHeader.Size > 2*1024*1024 { // SVG 很小，超 2MB 直接拒绝检查读取
		return fmt.Errorf("svg too large")
	}
	f, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 2*1024*1024))
	if err != nil {
		return err
	}
	content := strings.ToLower(string(data))
	if strings.Contains(content, "<script") ||
		strings.Contains(content, "onload=") ||
		strings.Contains(content, "onerror=") ||
		strings.Contains(content, "onclick=") ||
		strings.Contains(content, "javascript:") ||
		strings.Contains(content, "onmouseover=") {
		return fmt.Errorf("svg contains active content")
	}
	return nil
}
