package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gin-apeadmin/internal/core"
	"gin-apeadmin/internal/model"
	"gin-apeadmin/internal/pkg/response"
	"gin-apeadmin/internal/service"
)

// McpHandler MCP 管理 Handler
type McpHandler struct{}

// Tools 列出 MCP 工具（按权限过滤）
func (h *McpHandler) Tools(c *gin.Context) {
	manager := core.GetMCPManager()
	if manager == nil {
		c.JSON(http.StatusOK, response.Success([]interface{}{}))
		return
	}
	// 权限过滤：超管全量；普通用户按权限集合过滤
	var isSuper bool
	if userVal, exists := c.Get("user"); exists {
		isSuper = userVal.(*model.SysUser).IsSuperAdmin
	}
	var perms map[string]bool
	if !isSuper {
		perms = service.GetUserPermissions(c.GetUint("user_id"))
	}
	tools := manager.ListTools()
	result := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		allowed := isSuper
		if !allowed && len(t.RequiredPermissions) == 0 {
			allowed = true
		}
		if !allowed {
			for _, p := range t.RequiredPermissions {
				if perms[p] {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			continue
		}
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		}
		result = append(result, map[string]interface{}{
			"name":                t.Name,
			"description":         t.Description,
			"input_schema":        schema,
			"category":            t.Category,
			"plugin_name":         t.PluginName,
			"required_permissions": t.RequiredPermissions,
		})
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// ToolCategories 工具分类统计
func (h *McpHandler) ToolCategories(c *gin.Context) {
	manager := core.GetMCPManager()
	if manager == nil {
		c.JSON(http.StatusOK, response.Success([]interface{}{}))
		return
	}
	c.JSON(http.StatusOK, response.Success(manager.ListCategories()))
}

// ToolCall 调用 MCP 工具
func (h *McpHandler) ToolCall(c *gin.Context) {
	var req struct {
		Name      string                 `json:"name" binding:"required"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	manager := core.GetMCPManager()
	if manager == nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "MCP 管理器未初始化"))
		return
	}
	tool, ok := manager.GetTool(req.Name)
	if !ok {
		c.JSON(http.StatusNotFound, response.Error(404, "工具不存在"))
		return
	}
	userID := c.GetUint("user_id")
	username := c.GetString("username")
	argsJSON, _ := json.Marshal(req.Arguments)

	start := time.Now()
	result, err := tool.Handler(req.Arguments)
	duration := time.Since(start).Milliseconds()
	status := "success"
	if err != nil {
		status = "failed"
	}
	resultJSON, _ := json.Marshal(result)
	manager.WriteAuditLog(userID, username, "tool", req.Name, string(argsJSON), string(resultJSON), status, duration)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "调用失败: "+err.Error()))
		return
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{"result": result}))
}

// Resources 列出 MCP 资源
func (h *McpHandler) Resources(c *gin.Context) {
	manager := core.GetMCPManager()
	if manager == nil {
		c.JSON(http.StatusOK, response.Success([]interface{}{}))
		return
	}
	resources := manager.ListResources()
	result := make([]map[string]interface{}, 0, len(resources))
	for _, r := range resources {
		result = append(result, map[string]interface{}{
			"uri":         r.URI,
			"name":        r.Name,
			"description": r.Description,
			"mime_type":   r.MimeType,
		})
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// ResourceRead 读取 MCP 资源
func (h *McpHandler) ResourceRead(c *gin.Context) {
	uri := c.Query("uri")
	if uri == "" {
		c.JSON(http.StatusBadRequest, response.Error(400, "uri 参数必填"))
		return
	}
	manager := core.GetMCPManager()
	if manager == nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "MCP 管理器未初始化"))
		return
	}
	userID := c.GetUint("user_id")
	username := c.GetString("username")
	start := time.Now()
	content, ok := manager.ReadResource(uri)
	duration := time.Since(start).Milliseconds()
	status := "success"
	if !ok {
		status = "failed"
	}
	manager.WriteAuditLog(userID, username, "resource", uri, "", content, status, duration)
	if !ok {
		c.JSON(http.StatusNotFound, response.Error(404, "资源不存在或读取失败"))
		return
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{"uri": uri, "content": content}))
}

// Prompts 列出 MCP 提示词
func (h *McpHandler) Prompts(c *gin.Context) {
	manager := core.GetMCPManager()
	if manager == nil {
		c.JSON(http.StatusOK, response.Success([]interface{}{}))
		return
	}
	prompts := manager.ListPrompts()
	result := make([]map[string]interface{}, 0, len(prompts))
	for _, p := range prompts {
		result = append(result, map[string]interface{}{
			"name":        p.Name,
			"description": p.Description,
			"arguments":   p.Arguments,
		})
	}
	c.JSON(http.StatusOK, response.Success(result))
}

// PromptRender 渲染提示词
func (h *McpHandler) PromptRender(c *gin.Context) {
	var req struct {
		Name      string            `json:"name" binding:"required"`
		Arguments map[string]string `json:"arguments"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.Error(400, "参数错误"))
		return
	}
	manager := core.GetMCPManager()
	if manager == nil {
		c.JSON(http.StatusInternalServerError, response.Error(500, "MCP 管理器未初始化"))
		return
	}
	userID := c.GetUint("user_id")
	username := c.GetString("username")
	argsJSON, _ := json.Marshal(req.Arguments)
	start := time.Now()
	rendered, ok := manager.RenderPrompt(req.Name, req.Arguments)
	duration := time.Since(start).Milliseconds()
	status := "success"
	if !ok {
		status = "failed"
	}
	manager.WriteAuditLog(userID, username, "prompt", req.Name, string(argsJSON), rendered, status, duration)
	if !ok {
		c.JSON(http.StatusNotFound, response.Error(404, "提示词不存在"))
		return
	}
	c.JSON(http.StatusOK, response.Success(map[string]interface{}{"rendered": rendered}))
}

// AuditLogs MCP 审计日志列表（分页）
func (h *McpHandler) AuditLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	actionType := c.Query("action_type")
	keyword := c.Query("keyword")

	db := core.GetDB()
	query := db.Model(&model.SysMcpAuditLog{})
	if actionType != "" {
		query = query.Where("action_type = ?", actionType)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("target_name LIKE ? OR username LIKE ?", like, like)
	}
	var total int64
	query.Count(&total)
	var items []model.SysMcpAuditLog
	query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)

	// 转换 created_at 为前端期望格式
	type auditItem struct {
		ID            uint   `json:"id"`
		ActionType    string `json:"action_type"`
		TargetName    string `json:"target_name"`
		Arguments     string `json:"arguments"`
		ResultPreview string `json:"result_preview"`
		Status        string `json:"status"`
		UserID        uint   `json:"user_id"`
		Username      string `json:"username"`
		CreatedAt     string `json:"created_at"`
	}
	rows := make([]auditItem, 0, len(items))
	for _, it := range items {
		rows = append(rows, auditItem{
			ID:            it.ID,
			ActionType:    it.ActionType,
			TargetName:    it.TargetName,
			Arguments:     it.Arguments,
			ResultPreview: it.Result,
			Status:        it.Status,
			UserID:        it.UserID,
			Username:      it.Username,
			CreatedAt:     it.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, response.SuccessPage(rows, int64(total), page, pageSize))
}
