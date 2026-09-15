package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
)

// RequestID 请求 ID 中间件（X-Request-ID）
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = time.Now().Format("20060102-150405.000") + "-"
			rid += randomString(6)
		}
		c.Set("request_id", rid)
		c.Header("X-Request-ID", rid)
		c.Next()
	}
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	now := time.Now().UnixNano()
	for i := range b {
		b[i] = letters[int(now>>uint(i*7))%len(letters)]
	}
	return string(b)
}
