package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"apeadmin-gin/internal/pkg/response"
)

// Recovery panic 恢复中间件
//
// 关键约束：panic 必须落日志。此前实现只把堆栈塞进 gin.Context（全仓无消费者），
// 异常被完全吞掉，线上只能看到一句 "Internal Server Error"。
// 现在同时写 zap（bootstrap 已 ReplaceGlobals）与标准库日志兜底
// （单元测试/初始化早期 zap 为 no-op，仅靠 zap 仍会静默）。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				zap.L().Error("panic recovered",
					zap.Any("panic", err),
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("client_ip", c.ClientIP()),
					zap.ByteString("stack", stack),
				)
				log.Printf("[PANIC] %s %s panic=%v\n%s", c.Request.Method, c.Request.URL.Path, err, stack)

				// 保留 context 变量：操作日志等后续中间件可读取
				c.Set("panic_stack", string(stack))
				c.Set("panic_error", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, response.Error(500, "Internal Server Error"))
			}
		}()
		c.Next()
	}
}
