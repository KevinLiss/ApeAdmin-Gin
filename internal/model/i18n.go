package model

// SysLocale 语言注册表
// 后台「多语言管理」可增删改；新增一门语言（如 ja-JP）无需改代码，
// 只需在此登记 + 在 sys_translation 补齐该语言的翻译条目
type SysLocale struct {
	BaseModel
	Code      string `gorm:"uniqueIndex;size:20;not null" json:"code"` // 如 zh-CN / en-US / ja-JP
	Name      string `gorm:"size:50;not null" json:"name"`              // 语言自称：简体中文 / English / 日本語
	Icon      string `gorm:"size:50" json:"icon"`                       // 国旗 emoji 或图标类名
	IsEnabled int    `gorm:"default:1;not null" json:"is_enabled"`      // 1 启用 0 停用
	IsDefault int    `gorm:"default:0;not null" json:"is_default"`      // 1 默认语言（仅一门）
	Sort      int    `gorm:"default:0" json:"sort"`
}

func (SysLocale) TableName() string { return "sys_locale" }

// SysTranslation 翻译条目
// 按命名空间组织（common/menu/auth/system/mcp/ai/…），与前端 locales 文件一一对应；
// 联合唯一 (locale_code, namespace, key)
type SysTranslation struct {
	BaseModel
	LocaleCode string `gorm:"uniqueIndex:idx_trans_locale_ns_key;size:20;not null" json:"locale_code"`
	Namespace  string `gorm:"uniqueIndex:idx_trans_locale_ns_key;size:50;not null" json:"namespace"`
	Key        string `gorm:"uniqueIndex:idx_trans_locale_ns_key;size:150;not null" json:"key"`
	Value      string `gorm:"type:text" json:"value"`
}

func (SysTranslation) TableName() string { return "sys_translation" }
