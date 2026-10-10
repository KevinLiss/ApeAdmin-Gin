package dal

import (
	"apeadmin-gin/internal/model"

	"gorm.io/gorm"
)

// ─── 多语言管理 ───

// ListLocales 语言列表（含停用，管理端用）
func ListLocales() ([]model.SysLocale, error) {
	var locales []model.SysLocale
	err := gormDB.Order("sort").Order("id").Find(&locales).Error
	return locales, err
}

// ListEnabledLocales 已启用语言列表（公开接口用）
func ListEnabledLocales() ([]model.SysLocale, error) {
	var locales []model.SysLocale
	err := gormDB.Where("is_enabled = ?", 1).Order("sort").Order("id").Find(&locales).Error
	return locales, err
}

// GetLocale 按 code 查
func GetLocale(code string) (*model.SysLocale, error) {
	var l model.SysLocale
	err := gormDB.Where("code = ?", code).First(&l).Error
	return &l, err
}

// GetLocaleByID 按 id 查
func GetLocaleByID(id uint) (*model.SysLocale, error) {
	var l model.SysLocale
	err := gormDB.First(&l, id).Error
	return &l, err
}

// CreateLocale 新增语言
func CreateLocale(l *model.SysLocale) error {
	return gormDB.Create(l).Error
}

// UpdateLocale 更新语言
func UpdateLocale(l *model.SysLocale) error {
	return gormDB.Save(l).Error
}

// DeleteLocale 删除语言（同时清掉该语言的翻译条目）
func DeleteLocale(code string) error {
	return gormDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("code = ?", code).Delete(&model.SysLocale{}).Error; err != nil {
			return err
		}
		return tx.Where("locale_code = ?", code).Delete(&model.SysTranslation{}).Error
	})
}

// ResetDefaultLocale 只保留一门默认语言（设默认前调用）
func ResetDefaultLocale(id uint) error {
	return gormDB.Model(&model.SysLocale{}).
		Where("id <> ?", id).
		Update("is_default", 0).Error
}

// ─── 翻译条目 ───

// ListTranslations 按 locale（可选 namespace）列出翻译
func ListTranslations(localeCode, namespace string) ([]model.SysTranslation, error) {
	var list []model.SysTranslation
	q := gormDB.Where("locale_code = ?", localeCode)
	if namespace != "" {
		q = q.Where("namespace = ?", namespace)
	}
	err := q.Order("namespace").Order("key").Find(&list).Error
	return list, err
}

// ListAllTranslations 全量翻译（导出用）
func ListAllTranslations(localeCode string) ([]model.SysTranslation, error) {
	var list []model.SysTranslation
	err := gormDB.Where("locale_code = ?", localeCode).
		Order("namespace").Order("key").Find(&list).Error
	return list, err
}

// GetTranslation 单条
func GetTranslation(localeCode, namespace, key string) (*model.SysTranslation, error) {
	var t model.SysTranslation
	err := gormDB.Where("locale_code = ? AND namespace = ? AND key = ?",
		localeCode, namespace, key).First(&t).Error
	return &t, err
}

// UpsertTranslations 批量 upsert 翻译（存在则更新 value，不存在则插入）
func UpsertTranslations(items []model.SysTranslation) error {
	return gormDB.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			var count int64
			if err := tx.Model(&model.SysTranslation{}).
				Where("locale_code = ? AND namespace = ? AND key = ?",
					item.LocaleCode, item.Namespace, item.Key).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				if err := tx.Model(&model.SysTranslation{}).
					Where("locale_code = ? AND namespace = ? AND key = ?",
						item.LocaleCode, item.Namespace, item.Key).
					Update("value", item.Value).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// DeleteTranslation 删除单条翻译
func DeleteTranslation(id uint) error {
	return gormDB.Delete(&model.SysTranslation{}, id).Error
}

// TranslationsByLocale 按语言分组全量翻译（i18n_loader 加载用）
func TranslationsByLocale() (map[string][]model.SysTranslation, error) {
	var list []model.SysTranslation
	if err := gormDB.Find(&list).Error; err != nil {
		return nil, err
	}
	result := map[string][]model.SysTranslation{}
	for _, t := range list {
		result[t.LocaleCode] = append(result[t.LocaleCode], t)
	}
	return result, nil
}
