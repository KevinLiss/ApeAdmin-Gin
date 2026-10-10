package response

import (
	"github.com/gin-gonic/gin"

	"apeadmin-gin/internal/core"
)

// Body 统一响应体
type Body struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// PageData 分页响应数据
type PageData struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    interface{} `json:"items"`
}

// Success 成功响应
func Success(data interface{}) Body {
	return Body{Code: 200, Msg: "success", Data: data}
}

// SuccessMsg 成功响应（仅消息）
func SuccessMsg(msg string) Body {
	return Body{Code: 200, Msg: msg}
}

// Error 错误响应
func Error(code int, msg string) Body {
	return Body{Code: code, Msg: msg, Data: nil}
}

// ErrorT 错误响应（i18n）：按 Accept-Language 翻译 key
// params 用于 {name} 等占位符插值，如 ErrorT(404, locale, "common.not_found", map[string]string{"name": "角色"})
func ErrorT(code int, locale, key string, params map[string]string) Body {
	return Body{Code: code, Msg: core.T(locale, key, params), Data: nil}
}

// SuccessT 成功响应（i18n，仅消息）
func SuccessT(locale, key string, params map[string]string) Body {
	return Body{Code: 200, Msg: core.T(locale, key, params)}
}

// ctxLocale 从 gin.Context 取当前请求语言（Locale 中间件注入）
func ctxLocale(c *gin.Context) string {
	if c == nil {
		return core.DefaultLocale()
	}
	if v, ok := c.Get(core.CtxLocaleKey); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return core.DefaultLocale()
}

// ErrorCtx 错误响应（i18n，上下文版）：自动从 gin.Context 取语言
func ErrorCtx(c *gin.Context, code int, key string, params map[string]string) Body {
	return ErrorT(code, ctxLocale(c), key, params)
}

// SuccessCtx 成功响应（i18n，上下文版，仅消息）
func SuccessCtx(c *gin.Context, key string, params map[string]string) Body {
	return SuccessT(ctxLocale(c), key, params)
}

// SuccessPage 分页成功响应
func SuccessPage(items interface{}, total int64, page, pageSize int) Body {
	return Body{Code: 200, Msg: "success", Data: PageData{
		Total: total, Page: page, PageSize: pageSize, Items: items,
	}}
}
