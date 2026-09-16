package dal

import "apeadmin-gin/internal/model"

// ─── 日志 ───

func ListLogs(page, pageSize int) ([]model.SysLog, int64, error) {
	var logs []model.SysLog
	var total int64
	db := gormDB.Model(&model.SysLog{})
	db.Count(&total)
	err := db.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&logs).Error
	return logs, total, err
}

func GetLogByID(id uint) (*model.SysLog, error) {
	var log model.SysLog
	err := gormDB.First(&log, id).Error
	return &log, err
}

func ClearLogs() error {
	return gormDB.Where("1 = 1").Delete(&model.SysLog{}).Error
}

func DeleteLog(id uint) error {
	return gormDB.Delete(&model.SysLog{}, id).Error
}

// ─── 系统设置 ───

func ListSettings() ([]model.SysSetting, error) {
	var settings []model.SysSetting
	err := gormDB.Find(&settings).Error
	return settings, err
}

func ListPublicSettings() ([]model.SysSetting, error) {
	var settings []model.SysSetting
	err := gormDB.Where("is_public = ?", true).Find(&settings).Error
	return settings, err
}

func GetSetting(key string) (*model.SysSetting, error) {
	var setting model.SysSetting
	err := gormDB.Where("key = ?", key).First(&setting).Error
	return &setting, err
}

func UpdateSetting(key, value string) error {
	return gormDB.Model(&model.SysSetting{}).Where("key = ?", key).Update("value", value).Error
}

// BatchUpdateSettings 批量更新系统设置（upsert：存在则更新，不存在则创建）
// 品牌类 key（站点名称/Logo/主题色/页脚/登录背景/后台路径/侧边栏主题）新建时默认公开，
// 供登录页与前端 store 通过 /settings/public 读取；其余新 key 默认非公开。
func BatchUpdateSettings(settings map[string]string) error {
	tx := gormDB.Begin()
	for key, value := range settings {
		var count int64
		if err := tx.Model(&model.SysSetting{}).Where("key = ?", key).Count(&count).Error; err != nil {
			tx.Rollback()
			return err
		}
		if count > 0 {
			if err := tx.Model(&model.SysSetting{}).Where("key = ?", key).Update("value", value).Error; err != nil {
				tx.Rollback()
				return err
			}
		} else {
			// 新 key：品牌类公开，其余非公开
			isPublic := isBrandSettingKey(key)
			if err := tx.Create(&model.SysSetting{Key: key, Value: value, IsPublic: isPublic}).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit().Error
}

// brandSettingKeys 品牌/外观类设置 key，需通过公共接口暴露给未登录前端
var brandSettingKeys = map[string]bool{
	"site_name":      true,
	"logo_url":       true,
	"primary_color":  true,
	"footer_text":    true,
	"login_bg":       true,
	"admin_path":     true,
	"sidebar_theme":  true,
}

func isBrandSettingKey(key string) bool { return brandSettingKeys[key] }
