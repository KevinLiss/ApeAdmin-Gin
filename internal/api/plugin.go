package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/dal"
	"apeadmin-gin/internal/pkg/response"
	"apeadmin-gin/internal/plugin"
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

// Upload 上传 ZIP 插件包（L2 声明式清单插件）
func (h *PluginHandler) Upload(c *gin.Context) {
	cfg := core.GetConfig()
	if cfg == nil || !cfg.Plugin.Enabled {
		c.JSON(http.StatusBadRequest, response.Error(400, "插件功能未启用"))
		return
	}

	// 1. 接收 multipart 文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "缺少文件字段 file"))
		return
	}

	// 2. 扩展名校验
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".zip" {
		c.JSON(http.StatusBadRequest, response.Error(400, "仅支持 .zip 格式插件包"))
		return
	}

	// 3. 保存到临时目录
	tmpDir := filepath.Join(cfg.Plugin.UploadDir, "_tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "创建临时目录失败"))
		return
	}
	tmpPath := filepath.Join(tmpDir, fmt.Sprintf("%d-%s", time.Now().UnixNano(), file.Filename))
	if err := c.SaveUploadedFile(file, tmpPath); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "保存上传文件失败"))
		return
	}
	defer os.Remove(tmpPath) // 无论成功失败都清理临时文件

	// 4. 调用 L2 加载器
	manifest, err := plugin.LoadL2Plugin(tmpPath, plugin.LoaderConfig{
		UploadDir: cfg.Plugin.UploadDir,
		ZipGuard:  cfg.Plugin.ZipGuard,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.Success(gin.H{
		"refresh":      true,
		"name":         manifest.Name,
		"version":      manifest.Version,
		"display_name": manifest.DisplayName,
	}))
}

// Restart 重启后端（触发优雅关闭，由外部守护进程拉起）
func (h *PluginHandler) Restart(c *gin.Context) {
	core.RequestShutdown()
	c.JSON(http.StatusOK, response.Success(gin.H{
		"message": "服务正在重启...",
	}))
}

// Delete 删除插件
func (h *PluginHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	pluginRec, err := dal.GetPluginByID(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Error(404, "插件不存在"))
		return
	}

	// 记录文件路径后先删 DB 记录，再清理文件（文件清理失败仅记日志，不影响 DB）
	modulePath := pluginRec.ModulePath
	if err := dal.DeletePlugin(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "删除失败"))
		return
	}

	// 清理插件文件目录（ModulePath 或 uploads/plugins 下的插件目录）
	if modulePath != "" {
		if _, statErr := os.Stat(modulePath); statErr == nil {
			if rmErr := os.RemoveAll(modulePath); rmErr != nil {
				log.Printf("[plugin] 删除插件目录失败: %v (path=%s)", rmErr, modulePath)
			}
		}
	}

	c.JSON(http.StatusOK, response.Success(gin.H{"refresh": true}))
}
