package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gin-apeadmin/internal/core"
	"gin-apeadmin/internal/pkg/response"
)

// loginGuardEntry 登录失败记录
type loginGuardEntry struct {
	attempts  int
	firstFail time.Time
	lockedUntil time.Time
}

var (
	lgMu      sync.Mutex
	lgRecords = make(map[string]*loginGuardEntry) // key = ip + "|" + username
)

// LoginGuard 登录防爆破中间件（IP+用户名双滑窗计数锁定）
func LoginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := core.GetConfig().Security.LoginGuard
		ip := c.ClientIP()
		username := c.PostForm("username")
		if username == "" {
			// 尝试从 JSON body 读取
			var body map[string]interface{}
			if err := c.ShouldBindJSON(&body); err == nil {
				if u, ok := body["username"].(string); ok {
					username = u
				}
			}
		}

		key := ip + "|" + username
		lgMu.Lock()
		entry, exists := lgRecords[key]
		if exists && time.Now().Before(entry.lockedUntil) {
			lgMu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Error(429, "登录失败次数过多，请稍后再试"))
			return
		}
		lgMu.Unlock()
		c.Next()

		// 请求后检查是否登录失败
		if c.Writer.Status() == http.StatusUnauthorized && username != "" {
			lgMu.Lock()
			if !exists {
				entry = &loginGuardEntry{firstFail: time.Now()}
				lgRecords[key] = entry
			}
			entry.attempts++
			if entry.attempts >= cfg.MaxAttempts {
				entry.lockedUntil = time.Now().Add(time.Duration(cfg.LockMinutes) * time.Minute)
				entry.attempts = 0
			}
			lgMu.Unlock()
		} else if c.Writer.Status() == http.StatusOK {
			// 登录成功，清除记录
			lgMu.Lock()
			delete(lgRecords, key)
			lgMu.Unlock()
		}
	}
}
