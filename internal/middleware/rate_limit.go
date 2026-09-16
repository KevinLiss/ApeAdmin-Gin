package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/pkg/response"
)

// tokenBucket 令牌桶
type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

var (
	tbMu      sync.Mutex
	tbBuckets = make(map[string]*tokenBucket)
)

// RateLimit API 限流中间件（令牌桶，按 IP）
// name: 限流标识；rpm: 每分钟请求数；window: 时间窗口
func RateLimit(name string, rpm int, window time.Duration) gin.HandlerFunc {
 refillRate := float64(rpm) / window.Seconds()
 burst := rpm
 if burst < 1 {
  burst = 1
 }
	return func(c *gin.Context) {
		key := name + ":" + c.ClientIP()
		tbMu.Lock()
		bucket, exists := tbBuckets[key]
		if !exists {
			bucket = &tokenBucket{tokens: float64(burst), lastTime: time.Now()}
			tbBuckets[key] = bucket
		}
		// 补充令牌
		now := time.Now()
		elapsed := now.Sub(bucket.lastTime).Seconds()
		bucket.tokens += elapsed * refillRate
		if bucket.tokens > float64(burst) {
			bucket.tokens = float64(burst)
		}
		bucket.lastTime = now
		if bucket.tokens < 1 {
			tbMu.Unlock()
			retryAfter := int(1 / refillRate)
			if retryAfter < 1 { retryAfter = 1 }
			c.Header("Retry-After", time.Duration(retryAfter).String())
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Error(429, "请求过于频繁，请稍后再试"))
			return
		}
		bucket.tokens--
		tbMu.Unlock()
		c.Next()
	}
}
