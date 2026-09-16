package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/pkg/response"
)

// Recovery panic 恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				c.AbortWithStatusJSON(http.StatusInternalServerError, response.Error(500, "Internal Server Error"))
				// 打印堆栈到日志（生产环境用 zap）
				c.Set("panic_stack", string(stack))
				c.Set("panic_error", err)
			}
		}()
		c.Next()
	}
}
