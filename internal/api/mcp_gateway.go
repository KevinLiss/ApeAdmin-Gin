package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/mcp"
	"apeadmin-gin/internal/model"
	"apeadmin-gin/internal/service"

	"github.com/gin-gonic/gin"
)

// ────────────────────────────────────────────────────────────────────────────
// MCP 对外网关端点（Streamable HTTP / JSON-RPC 2.0）
//
// 外部 MCP 客户端经 OAuth 授权（或管理后台登录 token）后：
//   POST /mcp  → initialize / tools/list / tools/call / ping
//   GET  /mcp  → SSE 心跳流（保持连接；工具结果经 POST 同步返回）
//   DELETE /mcp → 断开
// 工具按调用者权限过滤（与 AI 全能助手同一套 RBAC），调用写审计日志。
// ────────────────────────────────────────────────────────────────────────────

// MCPGateway POST/GET/DELETE /mcp
func (h *McpHandler) MCPGateway(c *gin.Context) {
	// Origin 校验见 middleware.MCPOriginGuard（路由层，先于鉴权执行）
	switch c.Request.Method {
	case http.MethodPost:
		h.mcpHandlePost(c)
	case http.MethodGet:
		mcpHandleSSE(c)
	case http.MethodDelete:
		c.Status(http.StatusOK)
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
	}
}

func mcpJSONRPCResult(c *gin.Context, id interface{}, result interface{}) {
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": id, "result": result})
}

func mcpJSONRPCError(c *gin.Context, id interface{}, code int, message string) {
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": id, "error": gin.H{"code": code, "message": message}})
}

func (h *McpHandler) mcpHandlePost(c *gin.Context) {
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      interface{}     `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"jsonrpc": "2.0", "id": nil, "error": gin.H{"code": -32700, "message": "解析错误"}})
		return
	}
	// 通知类请求无 id → 202 空响应
	if req.ID == nil {
		c.Status(http.StatusAccepted)
		return
	}

	mgr := core.GetMCPManager()
	switch req.Method {
	case "initialize":
		version := "unknown"
		if cfg := core.GetConfig(); cfg != nil {
			version = cfg.App.Version
		}
		mcpJSONRPCResult(c, req.ID, gin.H{
			"protocolVersion": "2025-06-18",
			"capabilities":    gin.H{"tools": gin.H{"listChanged": false}},
			"serverInfo":      gin.H{"name": "ApeAdmin MCP", "version": version},
		})
	case "tools/list":
		if mgr == nil {
			mcpJSONRPCError(c, req.ID, -32603, "MCP 未初始化")
			return
		}
		perms := currentUserPerms(c)
		tools := []gin.H{}
		for _, t := range mgr.ListTools() {
			if !mcpToolAllowed(perms, t.RequiredPermissions) {
				continue
			}
			schema := map[string]interface{}{}
			_ = json.Unmarshal([]byte(mcp.MarshalSchema(t.InputSchema)), &schema)
			tools = append(tools, gin.H{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": schema,
			})
		}
		mcpJSONRPCResult(c, req.ID, gin.H{"tools": tools})
	case "tools/call":
		if mgr == nil {
			mcpJSONRPCError(c, req.ID, -32603, "MCP 未初始化")
			return
		}
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			mcpJSONRPCError(c, req.ID, -32602, "参数错误")
			return
		}
		mcpToolCall(c, req.ID, mgr, params.Name, params.Arguments)
	case "ping":
		mcpJSONRPCResult(c, req.ID, gin.H{})
	default:
		mcpJSONRPCError(c, req.ID, -32601, "未知方法: "+req.Method)
	}
}

// mcpToolCall 执行工具：权限校验 → 调用 → 审计
func mcpToolCall(c *gin.Context, id interface{}, mgr *mcp.Manager, name string, args map[string]interface{}) {
	entry, ok := mgr.GetTool(name)
	if !ok {
		mcpJSONRPCError(c, id, -32602, "工具不存在: "+name)
		return
	}
	perms := currentUserPerms(c)
	if !mcpToolAllowed(perms, entry.RequiredPermissions) {
		mcpJSONRPCError(c, id, -32603, "无权限调用该工具（需权限: "+fmt.Sprint(entry.RequiredPermissions)+"）")
		return
	}

	start := time.Now()
	result, err := entry.Handler(args)
	duration := time.Since(start).Milliseconds()
	status := "success"
	if err != nil {
		status = "failed"
	}
	// 审计（与 AI 助手工具调用同一张审计表）
	if v, ok := c.Get("user"); ok {
		if u, ok := v.(*model.SysUser); ok {
			argsJSON, _ := json.Marshal(args)
			resultJSON, _ := json.Marshal(result)
			mgr.WriteAuditLog(u.ID, u.Username, "tool", name, string(argsJSON), string(resultJSON), status, duration)
		}
	}

	if err != nil {
		mcpJSONRPCResult(c, id, gin.H{
			"content": []gin.H{{"type": "text", "text": "工具执行失败: " + err.Error()}},
			"isError": true,
		})
		return
	}
	text := ""
	if result != nil {
		if s, ok := result.(string); ok {
			text = s
		} else {
			data, _ := json.Marshal(result)
			text = string(data)
		}
	}
	mcpJSONRPCResult(c, id, gin.H{
		"content": []gin.H{{"type": "text", "text": text}},
		"isError": false,
	})
}

// mcpToolAllowed 权限校验（与 AI 助手工具调用一致的语义）
func mcpToolAllowed(perms map[string]bool, required []string) bool {
	if perms["*"] {
		return true
	}
	for _, p := range required {
		if !perms[p] {
			return false
		}
	}
	return true
}

// currentUserPerms 当前请求用户的权限集合（超管返回 *；与 AI 助手同源）
func currentUserPerms(c *gin.Context) map[string]bool {
	v, ok := c.Get("user")
	if !ok {
		return map[string]bool{}
	}
	user, ok := v.(*model.SysUser)
	if !ok {
		return map[string]bool{}
	}
	if user.IsSuperAdmin {
		return map[string]bool{"*": true}
	}
	return service.GetUserPermissionsCached(user.ID)
}

// mcpHandleSSE GET /mcp：保持连接的心跳流（工具调用经 POST 同步返回）
func mcpHandleSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.Status(http.StatusInternalServerError)
		return
	}
	ctx := c.Request.Context()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprint(c.Writer, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}
