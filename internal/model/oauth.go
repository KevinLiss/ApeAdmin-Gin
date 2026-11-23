package model

import "time"

// SysOAuthClient OAuth 客户端（外部 MCP 客户端接入：Claude/Cursor/自研 Agent 等）
// 动态客户端注册（RFC 7591）生成；管理端可停用/删除。
type SysOAuthClient struct {
	BaseLogModel
	Name             string `gorm:"size:100" json:"name"`
	ClientID         string `gorm:"size:64;uniqueIndex;not null" json:"client_id"`
	ClientSecretHash string `gorm:"size:100" json:"-"` // bcrypt；明文仅在注册响应中出现一次
	RedirectURIs     string `gorm:"type:text" json:"redirect_uris"` // JSON 数组
	Enabled          bool   `gorm:"default:true" json:"enabled"`
	Remark           string `gorm:"size:200" json:"remark"`
}

func (SysOAuthClient) TableName() string { return "sys_oauth_client" }

// SysOAuthCode 授权码（一次性，5 分钟有效；PKCE 绑定防截持）
type SysOAuthCode struct {
	BaseLogModel
	Code            string    `gorm:"size:128;uniqueIndex;not null" json:"-"`
	ClientID        string    `gorm:"size:64;index" json:"client_id"`
	UserID          uint      `gorm:"index" json:"user_id"`
	RedirectURI     string    `gorm:"type:text" json:"-"`
	Scope           string    `gorm:"size:100" json:"scope"`
	CodeChallenge   string    `gorm:"size:128" json:"-"`
	ChallengeMethod string    `gorm:"size:16" json:"-"`
	ExpiresAt       time.Time `gorm:"index" json:"expires_at"`
	Used            bool      `gorm:"default:false" json:"used"`
}

func (SysOAuthCode) TableName() string { return "sys_oauth_code" }

// SysOAuthRefreshToken MCP 客户端 refresh token（hash 存储，轮换式）
type SysOAuthRefreshToken struct {
	BaseLogModel
	TokenHash string    `gorm:"size:64;uniqueIndex;not null" json:"-"` // sha256 hex
	ClientID  string    `gorm:"size:64;index" json:"client_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	ExpiresAt time.Time `gorm:"index" json:"expires_at"`
	Revoked   bool      `gorm:"default:false;index" json:"revoked"`
}

func (SysOAuthRefreshToken) TableName() string { return "sys_oauth_refresh_token" }