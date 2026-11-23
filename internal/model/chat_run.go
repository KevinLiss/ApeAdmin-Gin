package model

import "time"

// SysChatRun AI 对话后台运行任务（qm 式 run 架构）
//
// 生命周期：pending → running → done | failed | stopped
// 每条用户消息对应一个 run，由后台 worker 执行 LLM 调用，
// 与 HTTP 请求生命周期解耦：关闭页面/切换路由不会中断生成。
type SysChatRun struct {
	BaseLogModel
	SessionID   uint       `gorm:"index;not null" json:"session_id"`
	UserID      uint       `gorm:"index;not null" json:"user_id"`
	Status      string     `gorm:"size:20;index;not null;default:pending" json:"status"`
	ProviderID  uint       `json:"provider_id"`
	Model       string     `gorm:"size:100" json:"model"`
	EnableTools bool       `gorm:"default:true" json:"enable_tools"`
	Question    string     `gorm:"type:text" json:"question"`
	Answer      string     `gorm:"type:text" json:"answer"`
	ToolEvents  string     `gorm:"type:text" json:"tool_events"`
	Error       string     `gorm:"type:text" json:"error"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	// 多实例支持：执行实例标识 + 领取心跳（单实例下恒为本实例，行为不变）
	ClaimedBy string     `gorm:"size:64;index" json:"claimed_by"`
	ClaimedAt *time.Time `json:"claimed_at"`
}

func (SysChatRun) TableName() string { return "sys_chat_run" }