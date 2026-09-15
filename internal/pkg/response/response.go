package response

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

// SuccessPage 分页成功响应
func SuccessPage(items interface{}, total int64, page, pageSize int) Body {
	return Body{Code: 200, Msg: "success", Data: PageData{
		Total: total, Page: page, PageSize: pageSize, Items: items,
	}}
}
