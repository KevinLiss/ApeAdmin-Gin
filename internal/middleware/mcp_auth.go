package middleware

import (
	"github.com/gin-gonic/gin"
)

// MCPAuth MCP 对外网关鉴权：复用 JWTAuth（标准 access token / OAuth 签发的 MCP token）。
// 预置 WWW-Authenticate 头：401 时客户端据此发现资源元数据（RFC 6750 / RFC 9728）。
func MCPAuth() gin.HandlerFunc {
	jwt := JWTAuth()
	return func(c *gin.Context) {
		c.Header("WWW-Authenticate",
			`Bearer resource_metadata="/.well-known/oauth-protected-resource"`)
		jwt(c)
	}
}
