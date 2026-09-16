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

func BatchUpdateSettings(settings map[string]string) error {
	tx := gormDB.Begin()
	for key, value := range settings {
		if err := tx.Model(&model.SysSetting{}).Where("key = ?", key).Update("value", value).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}
