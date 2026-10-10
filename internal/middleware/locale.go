package middleware

import (
	"github.com/gin-gonic/gin"

	"apeadmin-gin/internal/core"
)

// Locale 语言解析中间件
// 每个请求解析 Accept-Language 头（前端 axios 拦截器注入），
// 归一化后存入 context（键为 core.CtxLocaleKey）；未携带时使用系统默认语言
func Locale() gin.HandlerFunc {
	return func(c *gin.Context) {
		locale := core.ParseLocale(c.GetHeader("Accept-Language"))
		c.Set(core.CtxLocaleKey, locale)
		c.Next()
	}
}

// GetLocale 从 gin.Context 取出当前请求语言（middleware 包外调用）
func GetLocale(c *gin.Context) string {
	if v, ok := c.Get(core.CtxLocaleKey); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return core.DefaultLocale()
}
