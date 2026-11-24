package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"apeadmin-gin/internal/core"
)

// MCPOriginGuard MCP 网关 DNS rebinding 防护。
//
// MCP 网关挂在根路径 /mcp 并携带用户 token：若不做 Origin 校验，
// 恶意网页可借用户浏览器以 localhost 身份调用工具（MCP 规范要求校验 Origin）。
// 必须排在鉴权之前，否则未授权请求会先拿到 401，掩盖 Origin 拒绝行为。
func MCPOriginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin := c.GetHeader("Origin"); origin != "" && !MCPOriginAllowed(origin, c.Request.Host) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"jsonrpc": "2.0", "id": nil,
				"error": gin.H{"code": -32600, "message": "Origin 未授权"}})
			return
		}
		c.Next()
	}
}

// MCPOriginAllowed 校验浏览器 Origin：
//   - 同源（Origin 的 host 与请求 Host 一致）放行；
//   - 或在 cors.origins 白名单内（开发环境 vite 代理、分域部署）放行；
//   - 其余一律拒绝。无 Origin 头的非浏览器客户端（CLI/MCP SDK）不受影响。
func MCPOriginAllowed(origin, requestHost string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, requestHost) {
		return true
	}
	cfg := core.GetConfig()
	if cfg == nil {
		return false
	}
	for _, o := range cfg.CORS.Origins {
		if o == "*" {
			return true
		}
		if strings.EqualFold(strings.TrimRight(o, "/"), strings.TrimRight(origin, "/")) {
			return true
		}
	}
	return false
}
