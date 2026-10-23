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

func init() {
	// 后台 sweeper：定期清理不活跃的限流桶，防止按 IP 无限增长的内存泄漏
	go func() {
		for range time.Tick(10 * time.Minute) {
			tbMu.Lock()
			cutoff := time.Now().Add(-30 * time.Minute) // 30 分钟无活动的桶回收
			for key, b := range tbBuckets {
				if b.lastTime.Before(cutoff) {
					delete(tbBuckets, key)
				}
			}
			tbMu.Unlock()
		}
	}()
}

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
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.ErrorCtx(c, 429, "auth.rate_limited", nil))
			return
		}
		bucket.tokens--
		tbMu.Unlock()
		c.Next()
	}
}
