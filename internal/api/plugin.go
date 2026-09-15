package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gin-apeadmin/internal/dal"
	"gin-apeadmin/internal/pkg/response"
)

// PluginHandler 插件管理 API
type PluginHandler struct{}

// List 插件列表
func (h *PluginHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "12"))
	plugins, total, err := dal.ListPlugins(page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "查询失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessPage(plugins, total, page, pageSize))
}

// Toggle 启停插件
func (h *PluginHandler) Toggle(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	plugin, err := dal.GetPluginByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "插件不存在"))
		return
	}
	plugin.Enabled = req.Enabled
	if err := dal.UpdatePlugin(plugin); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "操作失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"refresh": true}))
}

// GetConfig 获取插件配置
func (h *PluginHandler) GetConfig(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	plugin, err := dal.GetPluginByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "插件不存在"))
		return
	}
	config := "{}"
	if plugin.Config != nil {
		config = *plugin.Config
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"config": config}))
}

// UpdateConfig 更新插件配置
func (h *PluginHandler) UpdateConfig(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	var req struct {
		Config string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	plugin, err := dal.GetPluginByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "插件不存在"))
		return
	}
	cfg := req.Config
	plugin.Config = &cfg
	if err := dal.UpdatePlugin(plugin); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "更新失败"))
		return
	}
	c.JSON(http.StatusOK, response.SuccessMsg("配置已保存"))
}

// Upload 上传 ZIP 插件包
func (h *PluginHandler) Upload(c *gin.Context) {
	// 插件上传逻辑在 plugin/loader_l2.go 中实现
	// 此处为占位，实际调用 plugin.HandleUpload
	c.JSON(http.StatusNotImplemented, response.Error(501, "插件上传功能正在开发中"))
}

// Restart 重启后端
func (h *PluginHandler) Restart(c *gin.Context) {
	// 一期不实现重启，返回提示
	c.JSON(http.StatusOK, response.Success(gin.H{
		"message": "重启功能将在后续版本实现",
	}))
}

// Delete 删除插件
func (h *PluginHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	plugin, err := dal.GetPluginByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "插件不存在"))
		return
	}
	// 删除插件目录（如果存在）
	_ = plugin
	if err := dal.DeletePlugin(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "删除失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(gin.H{"refresh": true}))
}
