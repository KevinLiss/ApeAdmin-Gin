package core

import (
	"log"

	"apeadmin-gin/internal/model"

	"gorm.io/gorm"
)

// LoadI18nFromDB 从 sys_translation / sys_locale 加载 DB 覆盖层
// 在启动（种子之后）与后台翻译保存后调用，实现「后台改文案 → 热生效」
func LoadI18nFromDB(db *gorm.DB) {
	if db == nil {
		return
	}

	// 1. 默认语言（sys_locale.is_default=1）
	var locales []model.SysLocale
	if err := db.Where("is_enabled = ?", 1).Order("sort").Find(&locales).Error; err != nil {
		log.Printf("i18n：加载语言列表失败: %v（使用内置默认）", err)
	} else {
		for _, l := range locales {
			if l.IsDefault == 1 {
				// SetI18nOverrides 里会带默认语言一起写，先记下
				defaultLocaleCode = l.Code
			}
		}
	}

	// 2. 翻译覆盖层
	overrides := map[string]map[string]string{}
	var translations []model.SysTranslation
	if err := db.Find(&translations).Error; err != nil {
		log.Printf("i18n：加载翻译条目失败: %v（仅用内置字典）", err)
	} else {
		for _, t := range translations {
			if overrides[t.LocaleCode] == nil {
				overrides[t.LocaleCode] = map[string]string{}
			}
			// namespace.key 作为最终 key（与前端组织一致）
			overrides[t.LocaleCode][t.Namespace+"."+t.Key] = t.Value
		}
	}

	SetI18nOverrides(overrides, defaultLocaleCode)
	log.Printf("i18n：已加载 %d 种语言、%d 条 DB 翻译覆盖", len(overrides), len(translations))
}

// defaultLocaleCode 包内暂存默认语言（LoadI18nFromDB 用）
var defaultLocaleCode = "zh-CN"
