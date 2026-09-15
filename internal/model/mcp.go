package model

// SysMcpTool MCP 工具注册表
type SysMcpTool struct {
	BaseModel
	Name        string  `gorm:"uniqueIndex;size:100;not null" json:"name"`
	Description string  `gorm:"size:500" json:"description"`
	InputSchema *string `gorm:"type:text" json:"input_schema"`
	PluginName  string  `gorm:"size:100;index" json:"plugin_name"`
	Enabled     bool    `gorm:"default:true" json:"enabled"`
}

func (SysMcpTool) TableName() string { return "sys_mcp_tool" }

// SysMcpAuditLog MCP 调用审计日志
type SysMcpAuditLog struct {
	BaseLogModel
	UserID     uint   `gorm:"index" json:"user_id"`
	Username   string `gorm:"size:50" json:"username"`
	ActionType string `gorm:"size:50" json:"action_type"`
	TargetName string `gorm:"size:100" json:"target_name"`
	Arguments  string `gorm:"type:text" json:"arguments"`
	Status     string `gorm:"size:20" json:"status"`
	Result     string `gorm:"type:text" json:"result"`
	DurationMs int64  `json:"duration_ms"`
}

func (SysMcpAuditLog) TableName() string { return "sys_mcp_audit_log" }

// SysAiProvider AI 供应商
type SysAiProvider struct {
	BaseModel
	Name         string  `gorm:"size:100;not null" json:"name"`
	ProviderType string  `gorm:"size:50;not null" json:"provider_type"`
	BaseURL      string  `gorm:"size:255" json:"base_url"`
	Models       *string `gorm:"type:text" json:"models"`
	ApiKeyEnc    string  `gorm:"size:500" json:"-"`
	Enabled      bool    `gorm:"default:true" json:"enabled"`
}

func (SysAiProvider) TableName() string { return "sys_ai_provider" }

// SysChatSession AI 对话会话
type SysChatSession struct {
	BaseModel
	Title      string `gorm:"size:200" json:"title"`
	ProviderID uint   `gorm:"index" json:"provider_id"`
	Model      string `gorm:"size:100" json:"model"`
	UserID     uint   `gorm:"index" json:"user_id"`
}

func (SysChatSession) TableName() string { return "sys_chat_session" }

// SysChatMessage 对话消息
type SysChatMessage struct {
	BaseLogModel
	SessionID uint   `gorm:"index;not null" json:"session_id"`
	Role      string `gorm:"size:20;not null" json:"role"`
	Content   string `gorm:"type:text" json:"content"`
}

func (SysChatMessage) TableName() string { return "sys_chat_message" }
