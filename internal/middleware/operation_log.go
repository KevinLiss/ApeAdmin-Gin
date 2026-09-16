package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"apeadmin-gin/internal/core"
	"apeadmin-gin/internal/model"
)

// OperationLog 操作日志采集中间件（异步队列投递）
func OperationLog() gin.HandlerFunc {
	queue := core.GetAuditQueue()
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 跳过非 API 路由与日志查询本身
		if !strings.HasPrefix(path, "/api/v1/") || strings.HasPrefix(path, "/api/v1/logs") {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()

		// 值拷贝，与 gin.Context 生命周期解耦
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		uid := uint(0)
		if v, ok := userID.(uint); ok { uid = v }
		uname := ""
		if v, ok := username.(string); ok { uname = v }

		entry := model.SysLog{
			UserID:     uid,
			Username:   uname,
			Method:     c.Request.Method,
			Path:       path,
			Params:     "", // 生产环境可按需记录
			StatusCode: c.Writer.Status(),
			DurationMs: time.Since(start).Milliseconds(),
			IP:         c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
		}

		if isSensitiveOp(path, c.Request.Method) {
			// 敏感操作同步写
			if queue != nil {
				queue.SyncWrite(entry)
			}
		} else {
			// 普通操作投递队列
			if queue != nil {
				queue.Enqueue(entry)
			}
		}
	}
}
