package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
func CORS(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := false
		for _, o := range origins {
			if o == "*" || o == origin {
				allowed = true
				break
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Logger 请求日志中间件（Zap）
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		// 简化版：生产环境用 zap 记录
		_ = time.Since(start)
		_ = c.Request.URL.Path
	}
}

// isSensitiveOp 判断是否为敏感操作（同步写日志不走队列）
func isSensitiveOp(path, method string) bool {
	if strings.HasPrefix(path, "/api/v1/logs") {
		return false
	}
	if method == "DELETE" || strings.Contains(path, "/upload") || strings.Contains(path, "/plugins") {
		return true
	}
	return false
}
