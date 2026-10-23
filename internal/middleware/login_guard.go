package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/pkg/response"
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

func init() {
	// 后台 sweeper：定期清理过期的登录防护记录
	// （锁定已过期且无后续失败的条目回收，防止公网恶意扫描撑爆内存）
	go func() {
		for range time.Tick(10 * time.Minute) {
			lgMu.Lock()
			now := time.Now()
			for key, e := range lgRecords {
				// 锁定期已过且 30 分钟无新失败 → 回收
				if now.After(e.lockedUntil) && now.Sub(e.firstFail) > 30*time.Minute {
					delete(lgRecords, key)
				}
			}
			lgMu.Unlock()
		}
	}()
}

// LoginGuard 登录防爆破中间件（IP+用户名双滑窗计数锁定）
func LoginGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := core.GetConfig().Security.LoginGuard
		ip := c.ClientIP()

		// 读取原始 body 并完整恢复，避免影响后续 handler 解析
		var username string
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// JSON 请求：从 body 解析 username；表单请求：走 PostForm
		if ct := c.ContentType(); ct != "" && !strings.HasPrefix(ct, "application/x-www-form-urlencoded") && !strings.HasPrefix(ct, "multipart/form-data") {
			var body map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &body); err == nil {
				if u, ok := body["username"].(string); ok {
					username = u
				}
			}
		} else {
			username = c.PostForm("username")
		}

		key := ip + "|" + username
		lgMu.Lock()
		entry, exists := lgRecords[key]
		if exists && time.Now().Before(entry.lockedUntil) {
			lgMu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.ErrorCtx(c, 429, "auth.login_too_many", nil))
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
