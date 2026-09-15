package schema

// CommonResponse 通用响应结构
type CommonResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// UserInfo 用户信息响应
type UserInfo struct {
	ID          uint      `json:"id"`
	Username    string    `json:"username"`
	Nickname    string    `json:"nickname"`
	Avatar      *string   `json:"avatar"`
	Permissions []string  `json:"permissions"`
	Menus       []MenuNode `json:"menus"`
	Roles       []string  `json:"roles"`
}

// MenuNode 菜单树节点
type MenuNode struct {
	ID         uint        `json:"id"`
	Name       string      `json:"name"`
	ParentID   uint        `json:"parent_id"`
	Type       string      `json:"type"`
	Path       string      `json:"path"`
	Component  string      `json:"component"`
	Permission string      `json:"permission"`
	Icon       string      `json:"icon"`
	Sort       int         `json:"sort"`
	Visible    int         `json:"visible"`
	Status     int         `json:"status"`
	Children   []MenuNode  `json:"children"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=100"`
}

// UpdateProfileRequest 更新个人资料
type UpdateProfileRequest struct {
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Password string `json:"password" binding:"required,min=6,max=100"`
}
